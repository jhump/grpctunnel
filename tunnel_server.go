package grpctunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fullstorydev/grpchan"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/jhump/grpctunnel/tunnelpb"
)

func serveTunnel(stream tunnelStreamServer, tunnelMetadata metadata.MD, clientAcceptsSettings bool, handlers grpchan.HandlerMap, isClosing func() bool) error {
	svr := &tunnelServer{
		stream:                stream,
		services:              handlers,
		clientAcceptsSettings: clientAcceptsSettings,
		isClosing:             isClosing,
		streams:               map[int64]*tunnelServerStream{},
		lastSeen:              -1,
	}
	return svr.serve(tunnelMetadata)
}

type tunnelStreamServer interface {
	Context() context.Context
	Send(*tunnelpb.ServerToClient) error
	Recv() (*tunnelpb.ClientToServer, error)
}

type tunnelServer struct {
	stream                tunnelStreamServer
	services              grpchan.HandlerMap
	clientAcceptsSettings bool
	isClosing             func() bool

	mu       sync.RWMutex
	streams  map[int64]*tunnelServerStream
	lastSeen int64
}

func (s *tunnelServer) serve(tunnelMetadata metadata.MD) error {
	if s.clientAcceptsSettings {
		go func() {
			_ = s.stream.Send(&tunnelpb.ServerToClient{
				StreamId: -1,
				Frame: &tunnelpb.ServerToClient_Settings{
					Settings: &tunnelpb.Settings{
						InitialWindowSize: initialWindowSize,
						SupportedProtocolRevisions: []tunnelpb.ProtocolRevision{
							tunnelpb.ProtocolRevision_REVISION_ZERO, tunnelpb.ProtocolRevision_REVISION_ONE,
						},
					},
				},
			})
		}()
	}

	ctx := context.WithValue(s.stream.Context(), tunnelMetadataIncomingContextKey{}, tunnelMetadata)
	ctx, cancel := context.WithCancel(ctx)
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
			if ok, err := s.createStream(ctx, in.StreamId, f.NewStream); err != nil {
				if !ok {
					return err
				}
				// we don't want to stall the receive loop as that could lead to
				// flow control deadlock, so send on a different goroutine
				go func() {
					st, _ := status.FromError(err)
					_ = s.stream.Send(&tunnelpb.ServerToClient{
						StreamId: in.StreamId,
						Frame: &tunnelpb.ServerToClient_CloseStream{
							CloseStream: &tunnelpb.CloseStream{
								Status: st.Proto(),
							},
						},
					})
				}()
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

// createStream creates a new stream with the given ID. It returns false if this frame represents
// a protocol error, in which case the tunnel channel will be aborted with the returned error. If
// it returns true, then the frame represents a valid protocol frame. If it returns true, but also
// a non-nil error, the stream will be immediately closed with the returned error, but the tunnel
// itself is still valid for subsequent RPCs. This will be the case, for example, if the requested
// method name is not implemented by the server.
func (s *tunnelServer) createStream(ctx context.Context, streamID int64, frame *tunnelpb.NewStream) (bool, error) {
	if s.isClosing() {
		return true, status.Errorf(codes.Unavailable, "server is shutting down")
	}

	if frame.ProtocolRevision != tunnelpb.ProtocolRevision_REVISION_ZERO &&
		frame.ProtocolRevision != tunnelpb.ProtocolRevision_REVISION_ONE {
		return true, status.Errorf(codes.Unavailable, "server does not support protocol revision %d", frame.ProtocolRevision)
	}
	noFlowControl := frame.ProtocolRevision == tunnelpb.ProtocolRevision_REVISION_ZERO

	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.streams[streamID]
	if ok {
		// stream already active!
		return false, fmt.Errorf("cannot create stream ID %d: already exists", streamID)
	}
	if streamID <= s.lastSeen {
		return false, fmt.Errorf("cannot create stream ID %d: that ID has already been used", streamID)
	}
	s.lastSeen = streamID

	if frame.MethodName[0] == '/' {
		frame.MethodName = frame.MethodName[1:]
	}
	parts := strings.SplitN(frame.MethodName, "/", 2)
	if len(parts) != 2 {
		return true, status.Errorf(codes.InvalidArgument, "%s is not a well-formed method name", frame.MethodName)
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
		return true, status.Errorf(codes.Unimplemented, "%s not implemented", frame.MethodName)
	}
	headers := fromProto(frame.RequestHeaders)
	ctx = metadata.NewIncomingContext(ctx, headers)
	var cancel context.CancelFunc
	if timeout, ok := timeoutFromHeaders(headers); ok {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	str := &tunnelServerStream{
		ctx:            ctx,
		cancel:         cancel,
		svr:            s,
		streamID:       streamID,
		method:         frame.MethodName,
		stream:         s.stream,
		isClientStream: isClientStream,
		isServerStream: isServerStream,
	}
	sendFunc := func(data []byte, totalSize uint32, first bool) error {
		if first {
			return s.stream.Send(&tunnelpb.ServerToClient{
				StreamId: streamID,
				Frame: &tunnelpb.ServerToClient_ResponseMessage{
					ResponseMessage: &tunnelpb.MessageData{
						Size: totalSize,
						Data: data,
					},
				},
			})
		}
		return s.stream.Send(&tunnelpb.ServerToClient{
			StreamId: streamID,
			Frame: &tunnelpb.ServerToClient_MoreResponseData{
				MoreResponseData: data,
			},
		})
	}
	if noFlowControl {
		str.sender = newSenderWithoutFlowControl(sendFunc)
		str.receiver = newReceiverWithoutFlowControl[tunnelpb.ClientToServerFrame](ctx)
	} else {
		str.sender = newSender(ctx, frame.InitialWindowSize, sendFunc)
		str.receiver = newReceiver(
			func(m tunnelpb.ClientToServerFrame) uint {
				switch m := m.(type) {
				case *tunnelpb.ClientToServer_RequestMessage:
					return uint(len(m.RequestMessage.Data))
				case *tunnelpb.ClientToServer_MoreRequestData:
					return uint(len(m.MoreRequestData))
				default:
					return 0
				}
			},
			func(windowUpdate uint32) {
				if str.loadHalfClosed() != nil {
					// stream already half-closed, no more data coming
					return
				}
				_ = s.stream.Send(&tunnelpb.ServerToClient{
					StreamId: streamID,
					Frame: &tunnelpb.ServerToClient_WindowUpdate{
						WindowUpdate: windowUpdate,
					},
				})
			},
			initialWindowSize,
		)
	}

	s.streams[streamID] = str
	str.ctx = grpc.NewContextWithServerTransportStream(str.ctx, (*tunnelServerTransportStream)(str))
	go str.serveStream(md, svc)
	return true, nil
}

func timeoutFromHeaders(headers metadata.MD) (time.Duration, bool) {
	vals := headers.Get("grpc-timeout")
	if len(vals) == 0 {
		return 0, false
	}
	timeoutStr := vals[len(vals)-1]
	if len(timeoutStr) < 2 {
		return 0, false
	}
	timeout, err := strconv.Atoi(timeoutStr[:len(timeoutStr)-1])
	if err != nil {
		return 0, false
	}
	duration := time.Duration(timeout)
	switch timeoutStr[len(timeoutStr)-1] {
	case 'H':
		return duration * time.Hour, true
	case 'M':
		return duration * time.Minute, true
	case 'S':
		return duration * time.Second, true
	case 'm':
		return duration * time.Millisecond, true
	case 'u':
		return duration * time.Microsecond, true
	case 'n':
		return duration * time.Nanosecond, true
	default:
		return 0, false
	}
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

type errHolder struct {
	error
}

type tunnelServerStream struct {
	ctx      context.Context
	cancel   context.CancelFunc
	svr      *tunnelServer
	streamID int64
	method   string
	stream   tunnelStreamServer

	isClientStream bool
	isServerStream bool

	sender     sender
	receiver   receiver[tunnelpb.ClientToServerFrame]
	halfClosed atomic.Pointer[errHolder]

	// for reading frames from channel, to read message data
	readMu  sync.Mutex
	readErr error

	// for sending frames to client
	writeMu     sync.Mutex
	numSent     uint32
	headers     metadata.MD
	trailers    metadata.MD
	sentHeaders bool
	closed      bool
}

func (st *tunnelServerStream) acceptClientFrame(frame tunnelpb.ClientToServerFrame) {
	if st == nil {
		// can happen if server decided that the stream ID was recently used
		// yet inactive -- it returns nil error but also nil stream, which
		// just discards incoming messages (we assume they arrive late, racing
		// with stream being closed)
		return
	}

	switch frame := frame.(type) {
	// NewStream handled in caller
	case *tunnelpb.ClientToServer_HalfClose:
		st.halfClose(io.EOF)

	case *tunnelpb.ClientToServer_Cancel:
		st.finishStream(context.Canceled)

	case *tunnelpb.ClientToServer_WindowUpdate:
		st.sender.updateWindow(frame.WindowUpdate)

	case nil:
		st.finishStream(errors.New("protocol error: unrecognized frame type"))

	default:
		if err := st.receiver.accept(frame); err != nil {
			st.finishStream(err)
		}
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

func (st *tunnelServerStream) loadHalfClosed() error {
	if val := st.halfClosed.Load(); val != nil {
		return val.error
	}
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
	if int64(len(b)) > math.MaxUint32 {
		return status.Errorf(codes.ResourceExhausted, "serialized message is too large: %d bytes > maximum %d bytes", len(b), math.MaxUint32)
	}

	return st.sender.send(b)
}

func (st *tunnelServerStream) RecvMsg(m interface{}) error {
	data, ok, err := st.readMsg()
	if err != nil {
		if !ok {
			st.finishStream(err)
		}
		return err
	}
	// TODO: support alternate codecs, compressors, etc
	return proto.Unmarshal(data, m.(proto.Message))
}

func (st *tunnelServerStream) readMsg() (data []byte, ok bool, err error) {
	st.readMu.Lock()
	defer st.readMu.Unlock()

	data, ok, err = st.readMsgLocked()
	if err == nil && !st.isClientStream {
		// no stream; so eagerly see if there's another message
		// and fail RPC if so (due to bad input)
		_, ok, err := st.readMsgLocked()
		if err == nil {
			err = status.Errorf(codes.InvalidArgument, "Already received request for non-client-stream method %s", st.method)
			st.readErr = err
			return nil, false, err
		}
		if err != io.EOF || !ok {
			return nil, ok, err
		}
	}

	return data, ok, err
}

func (st *tunnelServerStream) readMsgLocked() (data []byte, ok bool, err error) {
	if st.readErr != nil {
		return nil, true, st.readErr
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
			return nil, true, err
		}

		in, ok := st.receiver.dequeue()
		if !ok {
			var err error
			if halfClosedErr := st.halfClosed.Load(); halfClosedErr != nil {
				err = halfClosedErr.error
			}
			return nil, true, err
		}

		switch in := in.(type) {
		case *tunnelpb.ClientToServer_RequestMessage:
			if msgLen != -1 {
				return nil, false, status.Errorf(codes.InvalidArgument, "received redundant request message envelope")
			}
			msgLen = int(in.RequestMessage.Size)
			b = in.RequestMessage.Data
			if len(b) > msgLen {
				return nil, false, status.Errorf(codes.InvalidArgument, "received more data than indicated by request message envelope")
			}
			if len(b) == msgLen {
				return b, true, nil
			}

		case *tunnelpb.ClientToServer_MoreRequestData:
			if msgLen == -1 {
				return nil, false, status.Errorf(codes.InvalidArgument, "never received envelope for request message")
			}
			b = append(b, in.MoreRequestData...)
			if len(b) > msgLen {
				return nil, false, status.Errorf(codes.InvalidArgument, "received more data than indicated by request message envelope")
			}
			if len(b) == msgLen {
				return b, true, nil
			}

		default:
			return nil, false, status.Errorf(codes.InvalidArgument, "unrecognized frame type: %T", in)
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
	go func() {
		// In case context closes asynchronously via timeout,
		// we need to make sure receiver is closed promptly.
		<-st.ctx.Done()
		st.receiver.cancel()
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
	st.cancel()
	st.svr.removeStream(st.streamID)
	st.halfClose(err)

	st.writeMu.Lock()
	defer st.writeMu.Unlock()

	if st.closed {
		return
	}

	stat, _ := status.FromError(err)

	headers := st.headers
	sendHeaders := !st.sentHeaders
	if sendHeaders {
		st.sentHeaders = true
		st.headers = nil
	}
	// we don't want to block here because we can be called from the
	// receive loop and we're also holding a mutex, so send the close
	// message from a different goroutine
	trailers := st.trailers
	go func() {
		if sendHeaders {
			_ = st.stream.Send(&tunnelpb.ServerToClient{
				StreamId: st.streamID,
				Frame: &tunnelpb.ServerToClient_ResponseHeaders{
					ResponseHeaders: toProto(headers),
				},
			})
		}
		_ = st.stream.Send(&tunnelpb.ServerToClient{
			StreamId: st.streamID,
			Frame: &tunnelpb.ServerToClient_CloseStream{
				CloseStream: &tunnelpb.CloseStream{
					Status:           stat.Proto(),
					ResponseTrailers: toProto(trailers),
				},
			},
		})
	}()

	if sendHeaders {
		st.sentHeaders = true
		st.headers = nil
	}

	st.closed = true
	st.trailers = nil
}

func (st *tunnelServerStream) halfClose(err error) {
	if err == nil {
		err = io.EOF
	}
	if !st.halfClosed.CompareAndSwap(nil, &errHolder{err}) {
		// already closed
		return
	}
	st.receiver.close()
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
