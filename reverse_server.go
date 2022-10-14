package grpctunnel

import (
	"context"
	"io"
	"sync"

	"github.com/fullstorydev/grpchan"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/jhump/grpctunnel/tunnelpb"
)

type reverseTunnelServerState int

const (
	stateActive = reverseTunnelServerState(iota)
	stateClosing
	stateClosed
)

// ReverseTunnelServer is a server that can run on the client side of a gRPC
// connection, handling requests sent over a reverse tunnel.
//
// Callers must first call NewReverseTunnelServer to create a new instance.
// Then callers register server handlers with the server and use the Serve
// method to actually create a reverse tunnel and handle requests.
type ReverseTunnelServer struct {
	stub     tunnelpb.TunnelServiceClient
	handlers grpchan.HandlerMap

	mu        sync.Mutex
	instances map[tunnelpb.TunnelService_OpenReverseTunnelClient]struct{}
	wg        sync.WaitGroup
	state     reverseTunnelServerState
}

// NewReverseTunnelServer creates a new server that uses the given stub to
// create reverse tunnels.
func NewReverseTunnelServer(stub tunnelpb.TunnelServiceClient) *ReverseTunnelServer {
	return &ReverseTunnelServer{
		stub:      stub,
		handlers:  grpchan.HandlerMap{},
		instances: map[tunnelpb.TunnelService_OpenReverseTunnelClient]struct{}{},
	}
}

func (s *ReverseTunnelServer) RegisterService(desc *grpc.ServiceDesc, srv interface{}) {
	s.handlers.RegisterService(desc, srv)
}

// Serve creates a new reverse tunnel and handles incoming RPC requests that
// arrive over that reverse tunnel. Since this is a reverse tunnel, RPC requests
// are initiated by the server, and this end (the client) processes the requests
// and sends responses.
//
// The boolean return value indicates whether the tunnel was created or not. If
// false, the returned error indicates the reason the tunnel could not be
// created. If true, the tunnel could be created and requests could be serviced.
// In this case, the returned error indicates the reason the tunnel was stopped.
// This will be nil if the stream was closed by the other side of the tunnel
// (the server, acting as an RPC client, hanging up).
//
// Reasons for the tunnel ending abnormally include detection of invalid usage
// of the stream (RPC client sending references to invalid stream IDs or sending
// frames for a stream ID in improper order) or if the stream itself fails (for
// example, if there is a network disruption or the given context is cancelled).
//
// Callers may call this repeatedly, to create multiple, concurrent streams to
// the gRPC server associated with this reverse tunnel server's associated stub.
func (s *ReverseTunnelServer) Serve(ctx context.Context, opts ...grpc.CallOption) (error, bool) {
	stream, err := s.stub.OpenReverseTunnel(ctx, opts...)
	if err != nil {
		return err, false
	}
	stream = &halfCloseSafeReverseTunnel{TunnelService_OpenReverseTunnelClient: stream}
	if err := s.addInstance(stream); err != nil {
		return err, false
	}
	defer s.wg.Done()
	err = serveTunnel(stream, s.handlers, s.isClosing)
	if err == context.Canceled && ctx.Err() == nil && s.isClosed() {
		// If we get back a cancelled error, but the given context is not
		// cancelled and this server is closed, then the cancellation was
		// caused by the server stopping. In that case, no need to report
		// an error to caller.
		err = nil
	}
	return err, true
}

func (s *ReverseTunnelServer) addInstance(stream tunnelpb.TunnelService_OpenReverseTunnelClient) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state >= stateClosing {
		return status.Errorf(codes.Unavailable, "no channels ready")
	}
	s.wg.Add(1)
	if s.instances == nil {
		s.instances = map[tunnelpb.TunnelService_OpenReverseTunnelClient]struct{}{}
	}
	s.instances[stream] = struct{}{}
	return nil
}

func (s *ReverseTunnelServer) isClosing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state >= stateClosing
}

func (s *ReverseTunnelServer) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state >= stateClosed
}

// Stop shuts down the server immediately. On return, the server has returned
// and any on-going operations have been cancelled.
func (s *ReverseTunnelServer) Stop() {
	defer s.wg.Wait()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == stateClosed {
		// already stopping
		return
	}
	s.state = stateClosed
	// immediately stop all instances
	for stream := range s.instances {
		_ = stream.CloseSend()
	}
}

// GracefulStop initiates graceful shutdown and waits for the server to stop.
// During graceful shutdown, no new operations will be allowed to start, but
// existing operations may proceed. The server stops after all existing
// operations complete.
//
// To given existing operations a time limit, the caller can also arrange to
// call Stop after some deadline, which forcibly terminates existing operations.
func (s *ReverseTunnelServer) GracefulStop() {
	defer s.wg.Wait()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != stateActive {
		// already stopping
		return
	}
	s.state = stateClosing
	// we just set the flag to stop new streams, but allow
	// existing streams to finish
}

type halfCloseSafeReverseTunnel struct {
	tunnelpb.TunnelService_OpenReverseTunnelClient
	mu     sync.Mutex
	closed bool
}

func (h *halfCloseSafeReverseTunnel) isClosed() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.closed
}

func (h *halfCloseSafeReverseTunnel) CloseSend() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	return h.TunnelService_OpenReverseTunnelClient.CloseSend()
}

func (h *halfCloseSafeReverseTunnel) Send(msg *tunnelpb.ServerToClient) error {
	return h.SendMsg(msg)
}

func (h *halfCloseSafeReverseTunnel) SendMsg(m interface{}) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return io.EOF
	}
	return h.TunnelService_OpenReverseTunnelClient.SendMsg(m)
}