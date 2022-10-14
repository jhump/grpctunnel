package grpctunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/fullstorydev/grpchan"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/jhump/grpctunnel/tunnelpb"
)

const maxChunkSize = 16384

func serveTunnel(stream tunnelStreamServer, handlers grpchan.HandlerMap, isClosing func() bool) error {
	svr := &tunnelServer{
		stream:    stream,
		services:  handlers,
		isClosing: isClosing,
		streams:   map[int64]*tunnelServerStream{},
		lastSeen:  -1,
	}
	return svr.serve()
}

type tunnelStreamServer interface {
	Context() context.Context
	Send(*tunnelpb.ServerToClient) error
	Recv() (*tunnelpb.ClientToServer, error)
}

type tunnelServer struct {
	stream    tunnelStreamServer
	services  grpchan.HandlerMap
	isClosing func() bool

	mu       sync.RWMutex
	streams  map[int64]*tunnelServerStream
	lastSeen int64
}

func (s *tunnelServer) serve() error {
	ctx, cancel := context.WithCancel(s.stream.Context())
	defer cancel()
	for {
		in, err := s.stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		if f, ok := in.Frame.(*tunnelpb.ClientToServer_NewStream); ok {
			if err, ok := s.createStream(ctx, in.StreamId, f.NewStream); err != nil {
				if !ok {
					return err
				} else {
					st, _ := status.FromError(err)
					_ = s.stream.Send(&tunnelpb.ServerToClient{
						StreamId: in.StreamId,
						Frame: &tunnelpb.ServerToClient_CloseStream{
							CloseStream: &tunnelpb.CloseStream{
								Status: st.Proto(),
							},
						},
					})
				}
			}
			continue
		}

		str, err := s.getStream(in.StreamId)
		if err != nil {
			return err
		}
		str.acceptClientFrame(in.Frame)
	}
}

func (s *tunnelServer) createStream(ctx context.Context, streamID int64, frame *tunnelpb.NewStream) (error, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.streams[streamID]
	if ok {
		// stream already active!
		return fmt.Errorf("cannot create stream ID %d: already exists", streamID), false
	}
	if streamID <= s.lastSeen {
		return fmt.Errorf("cannot create stream ID %d: that ID has already been used", streamID), false
	}
	s.lastSeen = streamID

	if frame.MethodName[0] == '/' {
		frame.MethodName = frame.MethodName[1:]
	}
	parts := strings.SplitN(frame.MethodName, "/", 2)
	if len(parts) != 2 {
		return status.Errorf(codes.InvalidArgument, "%s is not a well-formed method name", frame.MethodName), true
	}
	var md interface{}
	sd, svc := s.services.QueryService(parts[0])
	if sd != nil {
		md = findMethod(sd, parts[1])
	}
	var isClientStream, isServerStream bool
	if streamDesc, ok := md.(*grpc.StreamDesc); ok {
		isClientStream, isServerStream = streamDesc.ClientStreams, streamDesc.ServerStreams
	}

	if md == nil {
		return status.Errorf(codes.Unimplemented, "%s not implemented", frame.MethodName), true
	}
	if s.isClosing() {
		return status.Errorf(codes.Unavailable, "server is shutting down"), true
	}

	ctx = metadata.NewIncomingContext(ctx, fromProto(frame.RequestHeaders))

	ch := make(chan tunnelpb.IsClientToServer_Frame, 1)
	str := &tunnelServerStream{
		ctx:            ctx,
		svr:            s,
		streamID:       streamID,
		method:         frame.MethodName,
		stream:         s.stream,
		isClientStream: isClientStream,
		isServerStream: isServerStream,
		readChan:       ch,
		ingestChan:     ch,
	}
	s.streams[streamID] = str
	str.ctx = grpc.NewContextWithServerTransportStream(str.ctx, (*tunnelServerTransportStream)(str))
	go str.serveStream(md, svc)
	return nil, true
}

func (s *tunnelServer) getStream(streamID int64) (*tunnelServerStream, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	target, ok := s.streams[streamID]
	if !ok {
		if streamID <= s.lastSeen {
			// used and disposed of stream; ignore subsequent frames
			return nil, nil
		}
		// stream never created!
		return nil, fmt.Errorf("received frame for stream ID %d: stream never created", streamID)
	}

	return target, nil
}

func (s *tunnelServer) removeStream(streamID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.streams, streamID)
}

func findMethod(sd *grpc.ServiceDesc, method string) interface{} {
	for i, md := range sd.Methods {
		if md.MethodName == method {
			return &sd.Methods[i]
		}
	}
	for i, md := range sd.Streams {
		if md.StreamName == method {
			return &sd.Streams[i]
		}
	}
	return nil
}

type tunnelServerStream struct {
	ctx      context.Context
	svr      *tunnelServer
	streamID int64
	method   string
	stream   tunnelStreamServer

	isClientStream bool
	isServerStream bool

	// for "ingesting" frames into channel, from receive loop
	ingestMu   sync.Mutex
	ingestChan chan<- tunnelpb.IsClientToServer_Frame
	halfClosed error

	// for reading frames from channel, to read message data
	readMu   sync.Mutex
	readChan <-chan tunnelpb.IsClientToServer_Frame
	readErr  error

	// for sending frames to client
	writeMu     sync.Mutex
	numSent     uint32
	headers     metadata.MD
	trailers    metadata.MD
	sentHeaders bool
	closed      bool
}

func (st *tunnelServerStream) acceptClientFrame(frame tunnelpb.IsClientToServer_Frame) {
	if st == nil {
		// can happen if server decided that the stream ID was recently used
		// yet inactive -- it returns nil error but also nil stream, which
		// just discards incoming messages (we assume they arrive late, racing
		// with stream being closed)
		return
	}

	switch frame.(type) {
	case *tunnelpb.ClientToServer_HalfClose:
		st.halfClose(io.EOF)
		return

	case *tunnelpb.ClientToServer_Cancel:
		st.finishStream(context.Canceled)
		return
	}

	st.ingestMu.Lock()
	defer st.ingestMu.Unlock()

	if st.halfClosed != nil {
		// stream is half closed -- ignore subsequent messages
		return
	}

	select {
	case st.ingestChan <- frame:
	case <-st.ctx.Done():
	}
}

func (st *tunnelServerStream) SetHeader(md metadata.MD) error {
	return st.setHeader(md, false)
}

func (st *tunnelServerStream) SendHeader(md metadata.MD) error {
	return st.setHeader(md, true)
}

func (st *tunnelServerStream) setHeader(md metadata.MD, send bool) error {
	st.writeMu.Lock()
	defer st.writeMu.Unlock()

	if st.sentHeaders {
		return errors.New("already sent headers")
	}
	if md != nil {
		st.headers = metadata.Join(st.headers, md)
	}
	if send {
		return st.sendHeadersLocked()
	}
	return nil
}

func (st *tunnelServerStream) sendHeadersLocked() error {
	err := st.stream.Send(&tunnelpb.ServerToClient{
		StreamId: st.streamID,
		Frame: &tunnelpb.ServerToClient_ResponseHeaders{
			ResponseHeaders: toProto(st.headers),
		},
	})
	st.sentHeaders = true
	st.headers = nil
	return err
}

func fromProto(md *tunnelpb.Metadata) metadata.MD {
	if md == nil {
		return nil
	}
	vals := metadata.MD{}
	for k, v := range md.Md {
		vals[k] = v.Val
	}
	return vals
}

func toProto(md metadata.MD) *tunnelpb.Metadata {
	vals := map[string]*tunnelpb.Metadata_Values{}
	for k, v := range md {
		vals[k] = &tunnelpb.Metadata_Values{Val: v}
	}
	return &tunnelpb.Metadata{Md: vals}
}

func (st *tunnelServerStream) SetTrailer(md metadata.MD) {
	_ = st.setTrailer(md)
}

func (st *tunnelServerStream) setTrailer(md metadata.MD) error {
	st.writeMu.Lock()
	defer st.writeMu.Unlock()

	if st.closed {
		return errors.New("already finished")
	}
	st.trailers = metadata.Join(st.trailers, md)
	return nil
}

func (st *tunnelServerStream) Context() context.Context {
	return st.ctx
}

func (st *tunnelServerStream) SendMsg(m interface{}) error {
	st.writeMu.Lock()
	defer st.writeMu.Unlock()

	if !st.sentHeaders {
		if err := st.sendHeadersLocked(); err != nil {
			return err
		}
	}

	if !st.isServerStream && st.numSent == 1 {
		return status.Errorf(codes.Internal, "Already sent response for non-server-stream method %s", st.method)
	}
	st.numSent++

	// TODO: support alternate codecs, compressors, etc
	b, err := proto.Marshal(m.(proto.Message))
	if err != nil {
		return err
	}

	i := 0
	for {
		if err := st.ctx.Err(); err != nil {
			return err
		}

		chunk := b
		if len(b) > maxChunkSize {
			chunk = b[:maxChunkSize]
		}

		if i == 0 {
			err = st.stream.Send(&tunnelpb.ServerToClient{
				StreamId: st.streamID,
				Frame: &tunnelpb.ServerToClient_ResponseMessage{
					ResponseMessage: &tunnelpb.MessageData{
						Size: int32(len(b)),
						Data: chunk,
					},
				},
			})
		} else {
			err = st.stream.Send(&tunnelpb.ServerToClient{
				StreamId: st.streamID,
				Frame: &tunnelpb.ServerToClient_MoreResponseData{
					MoreResponseData: chunk,
				},
			})
		}

		if err != nil {
			return err
		}

		if len(b) <= maxChunkSize {
			break
		}

		b = b[maxChunkSize:]
		i++
	}

	return nil
}

func (st *tunnelServerStream) RecvMsg(m interface{}) error {
	data, err, ok := st.readMsg()
	if err != nil {
		if !ok {
			st.finishStream(err)
		}
		return err
	}
	// TODO: support alternate codecs, compressors, etc
	return proto.Unmarshal(data, m.(proto.Message))
}

func (st *tunnelServerStream) readMsg() (data []byte, err error, ok bool) {
	st.readMu.Lock()
	defer st.readMu.Unlock()

	data, err, ok = st.readMsgLocked()
	if err == nil && !st.isClientStream {
		// no stream; so eagerly see if there's another message
		// and fail RPC if so (due to bad input)
		_, err, ok := st.readMsgLocked()
		if err == nil {
			err = status.Errorf(codes.InvalidArgument, "Already received request for non-client-stream method %s", st.method)
			st.readErr = err
			return nil, err, false
		}
		if err != io.EOF || !ok {
			return nil, err, ok
		}
	}

	return data, err, ok
}

func (st *tunnelServerStream) readMsgLocked() (data []byte, err error, ok bool) {
	if st.readErr != nil {
		return nil, st.readErr, true
	}

	defer func() {
		if err != nil {
			st.readErr = err
		}
	}()

	msgLen := -1
	var b []byte
	for {
		// if stream is canceled, return context error
		if err := st.ctx.Err(); err != nil {
			return nil, err, true
		}

		// otherwise, try to read request data, but interrupt if
		// stream is canceled or half-closed
		select {
		case <-st.ctx.Done():
			return nil, st.ctx.Err(), true

		case in, ok := <-st.readChan:
			if !ok {
				// don't need lock to read st.halfClosed; observing
				// input channel close provides safe visibility
				return nil, st.halfClosed, true
			}

			switch in := in.(type) {
			case *tunnelpb.ClientToServer_RequestMessage:
				if msgLen != -1 {
					return nil, status.Errorf(codes.InvalidArgument, "received redundant request message envelope"), false
				}
				msgLen = int(in.RequestMessage.Size)
				b = in.RequestMessage.Data
				if len(b) > msgLen {
					return nil, status.Errorf(codes.InvalidArgument, "received more data than indicated by request message envelope"), false
				}
				if len(b) == msgLen {
					return b, nil, true
				}

			case *tunnelpb.ClientToServer_MoreRequestData:
				if msgLen == -1 {
					return nil, status.Errorf(codes.InvalidArgument, "never received envelope for request message"), false
				}
				b = append(b, in.MoreRequestData...)
				if len(b) > msgLen {
					return nil, status.Errorf(codes.InvalidArgument, "received more data than indicated by request message envelope"), false
				}
				if len(b) == msgLen {
					return b, nil, true
				}

			default:
				return nil, status.Errorf(codes.InvalidArgument, "unrecognized frame type: %T", in), false
			}
		}
	}
}

func (st *tunnelServerStream) serveStream(md interface{}, srv interface{}) {
	var err error
	panicked := true // pessimistic assumption

	defer func() {
		if err == nil && panicked {
			err = status.Errorf(codes.Internal, "panic")
		}
		st.finishStream(err)
	}()

	switch md := md.(type) {
	case *grpc.MethodDesc:
		var resp interface{}
		resp, err = md.Handler(srv, st.ctx, st.RecvMsg, nil)
		if err == nil {
			err = st.SendMsg(resp)
		}
	case *grpc.StreamDesc:
		err = md.Handler(srv, st)
	default:
		err = status.Errorf(codes.Internal, "unknown type of method desc: %T", md)
	}
	// if we get here, we did not panic
	panicked = false
}

func (st *tunnelServerStream) finishStream(err error) {
	st.svr.removeStream(st.streamID)

	st.halfClose(err)

	st.writeMu.Lock()
	defer st.writeMu.Unlock()

	if st.closed {
		return
	}

	if !st.sentHeaders {
		_ = st.sendHeadersLocked()
	}

	stat, _ := status.FromError(err)
	_ = st.stream.Send(&tunnelpb.ServerToClient{
		StreamId: st.streamID,
		Frame: &tunnelpb.ServerToClient_CloseStream{
			CloseStream: &tunnelpb.CloseStream{
				Status:           stat.Proto(),
				ResponseTrailers: toProto(st.trailers),
			},
		},
	})

	st.closed = true
	st.trailers = nil
}

func (st *tunnelServerStream) halfClose(err error) {
	st.ingestMu.Lock()
	defer st.ingestMu.Unlock()

	if st.halfClosed != nil {
		// already closed
		return
	}

	if err == nil {
		err = io.EOF
	}
	st.halfClosed = err
	close(st.ingestChan)
}

type tunnelServerTransportStream tunnelServerStream

func (st *tunnelServerTransportStream) Method() string {
	return (*tunnelServerStream)(st).method
}

func (st *tunnelServerTransportStream) SetHeader(md metadata.MD) error {
	return (*tunnelServerStream)(st).SetHeader(md)
}

func (st *tunnelServerTransportStream) SendHeader(md metadata.MD) error {
	return (*tunnelServerStream)(st).SendHeader(md)
}

func (st *tunnelServerTransportStream) SetTrailer(md metadata.MD) error {
	return (*tunnelServerStream)(st).setTrailer(md)
}
