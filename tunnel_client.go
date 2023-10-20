package grpctunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/jhump/grpctunnel/tunnelpb"
)

// NewChannel creates a new channel for issues RPCs. The returned channel
// implements [grpc.ClientConnInterface], so it can be used to create stubs
// and issue other RPCs, which are all carried over a single tunnel stream
// opened using the given stub. The given context defines the lifetime of
// the stream and therefore of the channel; if the context times out or is
// cancelled, the channel will be closed.
func NewChannel(ctx context.Context, stub tunnelpb.TunnelServiceClient, opts ...grpc.CallOption) (TunnelChannel, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, grpctunnelNegotiateKey, grpctunnelNegotiateVal)
	stream, err := stub.OpenTunnel(ctx, opts...)
	if err != nil {
		return nil, err
	}
	respMD, err := stream.Header()
	if err != nil {
		return nil, err
	}
	vals := respMD.Get(grpctunnelNegotiateKey)
	serverSendsSettings := len(vals) > 0 && vals[0] == grpctunnelNegotiateVal
	reqMD, _ := metadata.FromOutgoingContext(stream.Context())
	stream = &threadSafeOpenTunnelClient{TunnelService_OpenTunnelClient: stream}
	return newTunnelChannel(stream, reqMD, serverSendsSettings, func(*tunnelChannel) { _ = stream.CloseSend() }), nil
}

func newReverseChannel(stream tunnelpb.TunnelService_OpenReverseTunnelServer, onClose func(*tunnelChannel)) *tunnelChannel {
	md, _ := metadata.FromIncomingContext(stream.Context())
	vals := md.Get(grpctunnelNegotiateKey)
	serverSendsSettings := len(vals) > 0 && vals[0] == grpctunnelNegotiateVal
	stream = &threadSafeOpenReverseTunnelServer{TunnelService_OpenReverseTunnelServer: stream}
	return newTunnelChannel(stream, md, serverSendsSettings, onClose)
}

type tunnelStreamClient interface {
	Context() context.Context
	Send(*tunnelpb.ClientToServer) error
	Recv() (*tunnelpb.ServerToClient, error)
}

// TunnelChannel is a special gRPC connection that uses a gRPC stream (a tunnel)
// as its transport.
//
// See NewChannel for using a forward tunnels. The TunnelServiceHandler provides
// methods for using reverse tunnels. Both directions use this same interface.
type TunnelChannel interface {
	grpc.ClientConnInterface

	// Close shuts down the channel, cancelling any outstanding operations and
	// making it unavailable for subsequent operations. For forward tunnels,
	// this also closes the underlying stream.
	//
	// Channels for forward tunnels are implicitly closed if the context used
	// to create the underlying stream is cancelled or times out.
	Close()
	// Context returns the context for this channel. This context is derived
	// from the context associated with the underlying stream.
	//
	// For forward tunnels, this is a client context. So it will include
	// outgoing metadata for the request headers that were used to open the
	// tunnel. For reverse tunnels, this is a server context. So that request
	// metadata will be available as incoming metadata.
	Context() context.Context
	// Done returns a channel that can be used to await the channel closing.
	Done() <-chan struct{}
	// Err returns the error that caused the channel to close. If the channel
	// is not yet closed, this will return nil.
	Err() error
}

type threadSafeOpenTunnelClient struct {
	sendMu sync.Mutex
	recvMu sync.Mutex
	tunnelpb.TunnelService_OpenTunnelClient
}

func (h *threadSafeOpenTunnelClient) CloseSend() error {
	h.sendMu.Lock()
	defer h.sendMu.Unlock()
	return h.TunnelService_OpenTunnelClient.CloseSend()
}

func (h *threadSafeOpenTunnelClient) Send(msg *tunnelpb.ClientToServer) error {
	h.sendMu.Lock()
	defer h.sendMu.Unlock()
	return h.TunnelService_OpenTunnelClient.Send(msg)
}

func (h *threadSafeOpenTunnelClient) SendMsg(msg interface{}) error {
	h.sendMu.Lock()
	defer h.sendMu.Unlock()
	return h.TunnelService_OpenTunnelClient.SendMsg(msg)
}

func (h *threadSafeOpenTunnelClient) Recv() (*tunnelpb.ServerToClient, error) {
	h.recvMu.Lock()
	defer h.recvMu.Unlock()
	return h.TunnelService_OpenTunnelClient.Recv()
}

func (h *threadSafeOpenTunnelClient) RecvMsg(msg interface{}) error {
	h.recvMu.Lock()
	defer h.recvMu.Unlock()
	return h.TunnelService_OpenTunnelClient.RecvMsg(msg)
}

type threadSafeOpenReverseTunnelServer struct {
	sendMu sync.Mutex
	recvMu sync.Mutex
	tunnelpb.TunnelService_OpenReverseTunnelServer
}

func (h *threadSafeOpenReverseTunnelServer) Send(msg *tunnelpb.ClientToServer) error {
	h.sendMu.Lock()
	defer h.sendMu.Unlock()
	return h.TunnelService_OpenReverseTunnelServer.Send(msg)
}

func (h *threadSafeOpenReverseTunnelServer) SendMsg(msg interface{}) error {
	h.sendMu.Lock()
	defer h.sendMu.Unlock()
	return h.TunnelService_OpenReverseTunnelServer.SendMsg(msg)
}

func (h *threadSafeOpenReverseTunnelServer) Recv() (*tunnelpb.ServerToClient, error) {
	h.recvMu.Lock()
	defer h.recvMu.Unlock()
	return h.TunnelService_OpenReverseTunnelServer.Recv()
}

func (h *threadSafeOpenReverseTunnelServer) RecvMsg(msg interface{}) error {
	h.recvMu.Lock()
	defer h.recvMu.Unlock()
	return h.TunnelService_OpenReverseTunnelServer.RecvMsg(msg)
}

type tunnelChannel struct {
	stream              tunnelStreamClient
	tunnelMetadata      metadata.MD
	serverSendsSettings bool
	ctx                 context.Context
	cancel              context.CancelFunc
	tearDown            func(*tunnelChannel)

	awaitSettings chan struct{}
	settings      *tunnelpb.Settings
	useRevision   tunnelpb.ProtocolRevision

	mu            sync.RWMutex
	streams       map[int64]*tunnelClientStream
	lastStreamID  int64
	streamCreated bool
	err           error
	finished      bool

	streamCreation sync.Mutex
}

func newTunnelChannel(stream tunnelStreamClient, tunnelMetadata metadata.MD, serverSendsSettings bool, tearDown func(*tunnelChannel)) *tunnelChannel {
	ctx, cancel := context.WithCancel(stream.Context())
	c := &tunnelChannel{
		stream:              stream,
		tunnelMetadata:      tunnelMetadata,
		serverSendsSettings: serverSendsSettings,
		ctx:                 ctx,
		cancel:              cancel,
		tearDown:            tearDown,
		streams:             map[int64]*tunnelClientStream{},
		awaitSettings:       make(chan struct{}),
	}
	go c.recvLoop()

	// make sure we've gotten settings from the server before we return
	select {
	case <-c.awaitSettings:
	case <-ctx.Done():
	}

	return c
}

func (c *tunnelChannel) Context() context.Context {
	return c.ctx
}

func (c *tunnelChannel) Done() <-chan struct{} {
	return c.ctx.Done()
}

func (c *tunnelChannel) Err() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	switch c.err {
	case nil:
		return c.ctx.Err()
	case io.EOF:
		return nil
	default:
		return c.err
	}
}

func (c *tunnelChannel) Close() {
	c.close(nil)
}

func (c *tunnelChannel) Invoke(ctx context.Context, methodName string, req, resp interface{}, opts ...grpc.CallOption) error {
	str, err := c.newStream(ctx, false, false, methodName, opts...)
	if err != nil {
		return err
	}
	if err := str.SendMsg(req); err != nil {
		return err
	}
	if err := str.CloseSend(); err != nil {
		return err
	}
	err = str.RecvMsg(resp)
	if err != nil {
		return err
	}
	// Make sure there are no more messages on the stream.
	// Allocate another response (to make sure this call to
	// RecvMsg can't modify the resp we already received).
	rv := reflect.Indirect(reflect.ValueOf(resp))
	extraResp := reflect.New(rv.Type()).Interface()
	extraErr := str.RecvMsg(extraResp)
	if extraErr == nil {
		// Doh!
		str.cancel()
		extraErr = status.Errorf(codes.Internal, "unary RPC returned >1 response message")
	}
	// make sure to give thread-safe visibility to any trailers
	// recorded via use of the grpc.Trailer call option.
	str.Trailer()

	if errors.Is(extraErr, io.EOF) {
		return nil
	}
	return extraErr
}

func (c *tunnelChannel) NewStream(ctx context.Context, desc *grpc.StreamDesc, methodName string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return c.newStream(ctx, desc.ClientStreams, desc.ServerStreams, methodName, opts...)
}

func (c *tunnelChannel) newStream(ctx context.Context, clientStreams, serverStreams bool, methodName string, opts ...grpc.CallOption) (*tunnelClientStream, error) {
	// this lock is only used here, and orders all calls to newStream sequentially
	// to make sure streams are created (and NewStream message sent) with IDs in
	// monotonic order.
	c.streamCreation.Lock()
	defer c.streamCreation.Unlock()

	str, md, err := c.allocateStream(ctx, clientStreams, serverStreams, methodName, opts)
	if err != nil {
		return nil, err
	}
	err = c.stream.Send(&tunnelpb.ClientToServer{
		StreamId: str.streamID,
		Frame: &tunnelpb.ClientToServer_NewStream{
			NewStream: &tunnelpb.NewStream{
				MethodName:        methodName,
				RequestHeaders:    toProto(md),
				ProtocolRevision:  c.useRevision,
				InitialWindowSize: initialWindowSize,
			},
		},
	})
	if err != nil {
		c.removeStream(str.streamID)
		return nil, err
	}
	go func() {
		// if context gets cancelled, make sure
		// we shut down the stream
		<-str.ctx.Done()
		str.cancelStream(str.ctx.Err())
	}()
	return str, nil
}

func (c *tunnelChannel) allocateStream(ctx context.Context, clientStreams, serverStreams bool, methodName string, opts []grpc.CallOption) (*tunnelClientStream, metadata.MD, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.finished {
		return nil, nil, errors.New("channel is closed")
	}

	if c.lastStreamID < 0 {
		return nil, nil, errors.New("all stream IDs exhausted (must create a new channel)")
	}

	c.streamCreated = true
	c.lastStreamID++
	streamID := c.lastStreamID
	if _, ok := c.streams[streamID]; ok {
		// should never happen... panic?
		return nil, nil, errors.New("next stream ID not available")
	}

	md, _ := metadata.FromOutgoingContext(ctx)
	var hdrs, tlrs []*metadata.MD
	pr, _ := peer.FromContext(c.ctx)
	authority := "<unknown>"
	isSecure := false
	if pr != nil {
		authority = pr.Addr.String()
		isSecure = pr.AuthInfo != nil
	}

	for _, opt := range opts {
		switch opt := opt.(type) {
		case grpc.HeaderCallOption:
			hdrs = append(hdrs, opt.HeaderAddr)

		case grpc.TrailerCallOption:
			tlrs = append(tlrs, opt.TrailerAddr)

		case grpc.PeerCallOption:
			if pr != nil {
				*opt.PeerAddr = *pr
			}

		case grpc.PerRPCCredsCallOption:
			if opt.Creds.RequireTransportSecurity() && !isSecure {
				return nil, nil, fmt.Errorf("per-RPC credentials %T cannot be used with insecure channel", opt.Creds)
			}

			mdVals, err := opt.Creds.GetRequestMetadata(ctx, fmt.Sprintf("tunnel://%s%s", authority, methodName))
			if err != nil {
				return nil, nil, err
			}
			for k, v := range mdVals {
				md.Append(k, v)
			}

		case *tunnelChannelCallOption:
			*opt.ch = c

			// TODO: custom codec and compressor support
			//case grpc.ContentSubtypeCallOption:
			//case grpc.CustomCodecCallOption:
			//case grpc.CompressorCallOption:
		}
	}

	ctx, cncl := context.WithCancel(ctx)
	ctx = context.WithValue(ctx, tunnelMetadataOutgoingContextKey{}, c.tunnelMetadata)
	ctx = context.WithValue(ctx, tunnelChannelContextKey{}, c)
	str := &tunnelClientStream{
		ctx:              ctx,
		cancel:           cncl,
		ch:               c,
		streamID:         streamID,
		method:           methodName,
		stream:           c.stream,
		headersTargets:   hdrs,
		trailersTargets:  tlrs,
		isClientStream:   clientStreams,
		isServerStream:   serverStreams,
		gotHeadersSignal: make(chan struct{}),
		doneSignal:       make(chan struct{}),
	}
	sendData := func(data []byte, totalSize uint32, first bool) error {
		if first {
			return c.stream.Send(&tunnelpb.ClientToServer{
				StreamId: streamID,
				Frame: &tunnelpb.ClientToServer_RequestMessage{
					RequestMessage: &tunnelpb.MessageData{
						Size: totalSize,
						Data: data,
					},
				},
			})
		}
		return c.stream.Send(&tunnelpb.ClientToServer{
			StreamId: streamID,
			Frame: &tunnelpb.ClientToServer_MoreRequestData{
				MoreRequestData: data,
			},
		})
	}
	if c.useRevision == tunnelpb.ProtocolRevision_REVISION_ZERO {
		str.sender = newSenderWithoutFlowControl(sendData)
		str.receiver = newReceiverWithoutFlowControl[tunnelpb.ServerToClientFrame](ctx)
	} else {
		str.sender = newSender(ctx, c.settings.InitialWindowSize, sendData)
		str.receiver = newReceiver(
			func(frame tunnelpb.ServerToClientFrame) uint {
				switch frame := frame.(type) {
				case *tunnelpb.ServerToClient_ResponseMessage:
					return uint(len(frame.ResponseMessage.Data))
				case *tunnelpb.ServerToClient_MoreResponseData:
					return uint(len(frame.MoreResponseData))
				default:
					return 0
				}
			},
			func(windowUpdate uint32) {
				if str.loadDone() != nil {
					// don't bother with window updates; no more data coming
					return
				}
				_ = c.stream.Send(&tunnelpb.ClientToServer{
					StreamId: streamID,
					Frame: &tunnelpb.ClientToServer_WindowUpdate{
						WindowUpdate: windowUpdate,
					},
				})
			},
			initialWindowSize,
		)
	}

	c.streams[streamID] = str

	return str, md, nil
}

func (c *tunnelChannel) recvLoop() {
	if c.serverSendsSettings {
		in, err := c.stream.Recv()
		if err != nil {
			c.close(fmt.Errorf("failed to read settings from server: %w", err))
			return
		}
		if in.StreamId != -1 {
			c.close(fmt.Errorf("protocol error: settings frame had bad stream ID (%d)", in.StreamId))
			return
		}
		settings, ok := in.Frame.(*tunnelpb.ServerToClient_Settings)
		if !ok {
			c.close(fmt.Errorf("protocol error: first frame was not settings (instead was %T)", in.Frame))
			return
		}
		var valid bool
		for _, rev := range settings.Settings.SupportedProtocolRevisions {
			switch rev {
			case tunnelpb.ProtocolRevision_REVISION_ZERO, tunnelpb.ProtocolRevision_REVISION_ONE:
				if rev > c.useRevision {
					// use highest version that both server and client supports
					c.useRevision = rev
				}
				valid = true
			}
		}
		if !valid {
			c.close(fmt.Errorf("protocol error: server does not support revision %s or %s (supported = %v)",
				tunnelpb.ProtocolRevision_REVISION_ZERO, tunnelpb.ProtocolRevision_REVISION_ONE, settings.Settings.SupportedProtocolRevisions))
			return
		}
		c.settings = settings.Settings
	}
	close(c.awaitSettings)

	for {
		in, err := c.stream.Recv()
		if err != nil {
			c.close(err)
			return
		}

		str, err := c.getStream(in.StreamId)
		if err != nil {
			c.close(err)
			return
		}
		str.acceptServerFrame(in.Frame)
	}
}

func (c *tunnelChannel) getStream(streamID int64) (*tunnelClientStream, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	target, ok := c.streams[streamID]
	if !ok {
		if c.streamCreated && streamID <= c.lastStreamID {
			// used and disposed of stream; ignore subsequent frames
			return nil, nil
		}
		// stream never created!
		return nil, fmt.Errorf("received frame for stream ID %d: stream never created", streamID)
	}

	return target, nil
}

func (c *tunnelChannel) removeStream(streamID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.streams != nil {
		delete(c.streams, streamID)
	}
}

func (c *tunnelChannel) close(err error) bool {
	if c.tearDown != nil {
		c.tearDown(c)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.finished {
		return false
	}

	defer c.cancel()

	c.finished = true
	if err == nil {
		err = io.EOF
	}
	c.err = err
	for _, st := range c.streams {
		st.cancel()
	}
	c.streams = nil
	return true
}

type tunnelClientStream struct {
	ctx      context.Context
	cancel   context.CancelFunc
	ch       *tunnelChannel
	streamID int64
	method   string
	stream   tunnelStreamClient

	headersTargets  []*metadata.MD
	trailersTargets []*metadata.MD

	isClientStream bool
	isServerStream bool

	sender   sender
	receiver receiver[tunnelpb.ServerToClientFrame]
	done     atomic.Pointer[errHolder]

	// for processing metadata frames, from receive loop
	metaMu           sync.Mutex
	gotHeaders       bool
	gotHeadersSignal chan struct{}
	headers          metadata.MD
	doneSignal       chan struct{}
	trailers         metadata.MD

	// for reading frames from channel, to read message data
	readMu  sync.Mutex
	readErr error

	// for sending frames to server
	writeMu    sync.Mutex
	numSent    uint32
	halfClosed bool
}

func (st *tunnelClientStream) Header() (metadata.MD, error) {
	// if we've already received headers, return them
	select {
	case <-st.gotHeadersSignal:
		return st.headers, nil
	default:
	}

	select {
	case <-st.gotHeadersSignal:
		return st.headers, nil
	case <-st.ctx.Done():
		// in the event of a race, always respect getting headers first
		select {
		case <-st.gotHeadersSignal:
			return st.headers, nil
		default:
		}
		return nil, st.ctx.Err()
	}
}

func (st *tunnelClientStream) Trailer() metadata.MD {
	// Unlike Header(), this method does not block and should only be
	// used by client after stream is closed.
	select {
	case <-st.doneSignal:
		return st.trailers
	default:
		return nil
	}
}

func (st *tunnelClientStream) CloseSend() error {
	st.writeMu.Lock()
	defer st.writeMu.Unlock()

	select {
	case <-st.doneSignal:
		return st.loadDone()
	default:
		// don't block since we are holding writeMu
	}

	if st.halfClosed {
		return errors.New("already half-closed")
	}
	st.halfClosed = true
	return st.stream.Send(&tunnelpb.ClientToServer{
		StreamId: st.streamID,
		Frame: &tunnelpb.ClientToServer_HalfClose{
			HalfClose: &emptypb.Empty{},
		},
	})
}

func (st *tunnelClientStream) loadDone() error {
	if val := st.done.Load(); val != nil {
		return val.error
	}
	return nil
}

func (st *tunnelClientStream) Context() context.Context {
	return st.ctx
}

func (st *tunnelClientStream) SendMsg(m interface{}) error {
	st.writeMu.Lock()
	defer st.writeMu.Unlock()

	if !st.isClientStream && st.numSent == 1 {
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

func (st *tunnelClientStream) RecvMsg(m interface{}) error {
	data, ok, err := st.readMsg()
	if err != nil {
		if !ok {
			st.cancelStream(err)
		}
		return err
	}
	// TODO: support alternate codecs, compressors, etc
	return proto.Unmarshal(data, m.(proto.Message))
}

func (st *tunnelClientStream) readMsg() (data []byte, ok bool, err error) {
	st.readMu.Lock()
	defer st.readMu.Unlock()

	data, ok, err = st.readMsgLocked()
	if err == nil && !st.isServerStream {
		// no stream; so eagerly see if there's another message
		// and fail RPC if so (due to bad input)
		_, ok, err := st.readMsgLocked()
		if err == nil {
			err = status.Errorf(codes.Internal, "Server sent multiple responses for non-server-stream method %s", st.method)
			st.readErr = err
			return nil, false, err
		}
		if err != io.EOF || !ok {
			return nil, ok, err
		}
	}

	return data, ok, err
}

func (st *tunnelClientStream) readMsgLocked() (data []byte, ok bool, err error) {
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
		in, ok := st.receiver.dequeue()
		if !ok {
			return nil, true, st.loadDone()
		}

		switch in := in.(type) {
		case *tunnelpb.ServerToClient_ResponseMessage:
			if msgLen != -1 {
				return nil, false, status.Errorf(codes.Internal, "server sent redundant response message envelope")
			}
			msgLen = int(in.ResponseMessage.Size)
			b = in.ResponseMessage.Data
			if len(b) > msgLen {
				return nil, false, status.Errorf(codes.Internal, "server sent more data than indicated by response message envelope")
			}
			if len(b) == msgLen {
				return b, true, nil
			}

		case *tunnelpb.ServerToClient_MoreResponseData:
			if msgLen == -1 {
				return nil, false, status.Errorf(codes.Internal, "server never sent envelope for response message")
			}
			b = append(b, in.MoreResponseData...)
			if len(b) > msgLen {
				return nil, false, status.Errorf(codes.Internal, "server sent more data than indicated by response message envelope")
			}
			if len(b) == msgLen {
				return b, true, nil
			}

		default:
			return nil, false, status.Errorf(codes.Internal, "unrecognized frame type: %T", in)
		}
	}
}

func (st *tunnelClientStream) acceptServerFrame(frame tunnelpb.ServerToClientFrame) {
	if st == nil {
		// can happen if client decided that the stream ID was recently used
		// yet inactive -- it returns nil error but also nil stream, which
		// just discards incoming messages (we assume they arrive late, racing
		// with stream being closed)
		return
	}

	switch frame := frame.(type) {
	case *tunnelpb.ServerToClient_Settings:
		st.finishStream(errors.New("protocol error: unexpected settings frame"), nil)

	case *tunnelpb.ServerToClient_ResponseHeaders:
		st.metaMu.Lock()
		defer st.metaMu.Unlock()
		if st.gotHeaders {
			// TODO: cancel RPC and fail locally with internal error?
			return
		}
		st.gotHeaders = true
		st.headers = fromProto(frame.ResponseHeaders)
		for _, hdrs := range st.headersTargets {
			*hdrs = st.headers
		}
		close(st.gotHeadersSignal)

	case *tunnelpb.ServerToClient_CloseStream:
		trailers := fromProto(frame.CloseStream.ResponseTrailers)
		err := status.FromProto(frame.CloseStream.Status).Err()
		st.finishStream(err, trailers)

	case *tunnelpb.ServerToClient_WindowUpdate:
		st.sender.updateWindow(frame.WindowUpdate)

	case nil:
		st.finishStream(errors.New("protocol error: unrecognized frame type"), nil)

	default:
		if err := st.receiver.accept(frame); err != nil {
			st.finishStream(err, nil)
		}
	}
}

func (st *tunnelClientStream) cancelStream(err error) {
	if !st.finishStream(err, nil) {
		// stream already closed
		return
	}
	st.receiver.cancel()
	// Let server know, too.
	go func() {
		_ = st.stream.Send(&tunnelpb.ClientToServer{
			StreamId: st.streamID,
			Frame: &tunnelpb.ClientToServer_Cancel{
				Cancel: &emptypb.Empty{},
			},
		})
	}()
}

func (st *tunnelClientStream) finishStream(err error, trailers metadata.MD) bool {
	switch err {
	case nil:
		err = io.EOF
	case context.DeadlineExceeded:
		err = status.Error(codes.DeadlineExceeded, err.Error())
	case context.Canceled:
		err = status.Error(codes.Canceled, err.Error())
	}
	if !st.done.CompareAndSwap(nil, &errHolder{err}) {
		// done already set? then RPC already finished
		return false
	}
	defer st.cancel()
	st.ch.removeStream(st.streamID)
	st.receiver.close()

	st.metaMu.Lock()
	defer st.metaMu.Unlock()

	st.trailers = trailers
	for _, tlrs := range st.trailersTargets {
		*tlrs = trailers
	}
	if !st.gotHeaders {
		st.gotHeaders = true
		close(st.gotHeadersSignal)
	}
	close(st.doneSignal)

	return true
}
