package grpctunnel

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/fullstorydev/grpchan"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/jhump/grpctunnel/tunnelpb"
)

// TunnelServiceHandler provides an implementation for TunnelServiceServer. You
// can register handlers with it, and it will then expose those handlers for
// incoming tunnels. If no handlers are registered, the server will reply to
// OpenTunnel requests with an "Unimplemented" error code. The server may still
// be used for reverse tunnels
//
// For reverse tunnels, if supported, all connected channels (e.g. all clients
// that have created reverse tunnels) are available. You can also configure a
// listener to receive notices when channels are connected and disconnected.
//
// See NewTunnelServiceHandler.
type TunnelServiceHandler struct {
	handlers                  grpchan.HandlerMap
	noReverseTunnels          bool
	onReverseTunnelConnect    func(ReverseTunnelChannel)
	onReverseTunnelDisconnect func(ReverseTunnelChannel)
	affinityKey               func(ReverseTunnelChannel) any

	stopping atomic.Bool
	reverse  reverseChannels

	mu           sync.RWMutex
	reverseByKey map[interface{}]*reverseChannels
}

// TunnelServiceHandlerOptions contains various fields that can be used to
// customize a TunnelServiceHandler.
//
// See NewTunnelServiceHandler.
type TunnelServiceHandlerOptions struct {
	// If set, reverse tunnels will not be allowed. The server will reply to
	// OpenReverseTunnel requests with an "Unimplemented" error code.
	NoReverseTunnels bool
	// If reverse tunnels are allowed, this callback may be configured to
	// receive information when clients open a reverse tunnel.
	OnReverseTunnelConnect func(ReverseTunnelChannel)
	// If reverse tunnels are allowed, this callback may be configured to
	// receive information when reverse tunnels are torn down.
	OnReverseTunnelDisconnect func(ReverseTunnelChannel)
	// Optional function that accepts a reverse tunnel and returns an affinity
	// key. The affinity key values can be used to look up outbound channels,
	// for targeting calls to particular clients or groups of clients.
	AffinityKey func(ReverseTunnelChannel) any
}

// NewTunnelServiceHandler creates a new TunnelServiceHandler. The options are
// used to configure behavior for reverse tunnels. The returned handler is also
// a [grpc.ServiceRegistrar], to register the services that will be available
// for forward tunnels.
//
// The handler's Service method can be used to actually register the handler
// with a *grpc.Server.
func NewTunnelServiceHandler(options TunnelServiceHandlerOptions) *TunnelServiceHandler {
	return &TunnelServiceHandler{
		handlers:                  grpchan.HandlerMap{},
		noReverseTunnels:          options.NoReverseTunnels,
		onReverseTunnelConnect:    options.OnReverseTunnelConnect,
		onReverseTunnelDisconnect: options.OnReverseTunnelDisconnect,
		affinityKey:               options.AffinityKey,
		reverseByKey:              map[interface{}]*reverseChannels{},
	}
}

var _ grpc.ServiceRegistrar = (*TunnelServiceHandler)(nil)

func (s *TunnelServiceHandler) RegisterService(desc *grpc.ServiceDesc, srv interface{}) {
	s.handlers.RegisterService(desc, srv)
}

// Service returns the actual tunnel service implementation to register with a
// [grpc.ServiceRegistrar].
func (s *TunnelServiceHandler) Service() tunnelpb.TunnelServiceServer {
	return &tunnelServiceHandler{
		h: s,
	}
}

// InitiateShutdown starts the graceful shutdown process and returns
// immediately. This should be called when the server wants to shut down. This
// complements the normal process initiated by calling the GracefulStop method
// of a *grpc.Server. It prevents new operations from being initiated on any
// existing tunnel (while the main server's GracefulStop method prevents new
// tunnels from being established). This allows the server to drain, letting
// existing operations to complete.
func (s *TunnelServiceHandler) InitiateShutdown() {
	s.stopping.Store(true)
}

func (s *TunnelServiceHandler) openTunnel(stream tunnelpb.TunnelService_OpenTunnelServer) error {
	if len(s.handlers) == 0 {
		return status.Error(codes.Unimplemented, "forward tunnels not supported")
	}

	return serveTunnel(stream, s.handlers, s.stopping.Load)
}

func (s *TunnelServiceHandler) openReverseTunnel(stream tunnelpb.TunnelService_OpenReverseTunnelServer) error {
	if s.noReverseTunnels {
		return status.Error(codes.Unimplemented, "reverse tunnels not supported")
	}

	ch := newReverseChannel(stream)
	defer ch.Close()

	var key interface{}
	if s.affinityKey != nil {
		key = s.affinityKey(ch)
	}

	s.reverse.add(ch)
	defer s.reverse.remove(ch)

	rc := func() *reverseChannels {
		s.mu.Lock()
		defer s.mu.Unlock()

		rc := s.reverseByKey[key]
		if rc == nil {
			rc = &reverseChannels{}
			s.reverseByKey[key] = rc
		}
		return rc
	}()
	rc.add(ch)
	defer rc.remove(ch)

	if s.onReverseTunnelConnect != nil {
		s.onReverseTunnelConnect(ch)
	}
	if s.onReverseTunnelDisconnect != nil {
		defer s.onReverseTunnelDisconnect(ch)
	}

	<-ch.Done()
	return ch.Err()
}

type tunnelServiceHandler struct {
	tunnelpb.UnimplementedTunnelServiceServer
	h *TunnelServiceHandler
}

func (s *tunnelServiceHandler) OpenTunnel(stream tunnelpb.TunnelService_OpenTunnelServer) error {
	return s.h.openTunnel(stream)
}

func (s *tunnelServiceHandler) OpenReverseTunnel(stream tunnelpb.TunnelService_OpenReverseTunnelServer) error {
	return s.h.openReverseTunnel(stream)
}

type reverseChannels struct {
	mu    sync.Mutex
	chans []*reverseTunnelChannel
	idx   int
}

func (c *reverseChannels) allChans() []ReverseTunnelChannel {
	c.mu.Lock()
	defer c.mu.Unlock()

	cp := make([]ReverseTunnelChannel, len(c.chans))
	for i := range c.chans {
		cp[i] = c.chans[i]
	}
	return cp
}

func (c *reverseChannels) pick() grpc.ClientConnInterface {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.chans) == 0 {
		return nil
	}
	c.idx++
	if c.idx >= len(c.chans) {
		c.idx = 0
	}
	return c.chans[c.idx]
}

func (c *reverseChannels) add(ch *reverseTunnelChannel) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.chans = append(c.chans, ch)
}

func (c *reverseChannels) remove(ch *reverseTunnelChannel) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.chans {
		if c.chans[i] == ch {
			c.chans = append(c.chans[:i], c.chans[i+1:]...)
			break
		}
	}
}

// AllReverseTunnels returns the set of all currently active reverse tunnels.
func (s *TunnelServiceHandler) AllReverseTunnels() []ReverseTunnelChannel {
	return s.reverse.allChans()
}

// AsChannel returns a channel that can be used for issuing RPCs back to clients
// over reverse tunnels. If no reverse tunnels are established, RPCs will fail
// with "Unavailable" errors.
//
// The returned channel will use a round-robin strategy to select from available
// reverse tunnels for any given RPC.
//
// This method panics if the handler was created with an option to disallow the
// use of reverse tunnels.
func (s *TunnelServiceHandler) AsChannel() grpc.ClientConnInterface {
	if s.noReverseTunnels {
		panic("reverse tunnels not supported")
	}
	return multiChannel(s.reverse.pick)
}

// KeyAsChannel returns a channel that can be used for issuing RPCs back to
// clients over reverse tunnels whose affinity key matches the given value.
// If no reverse tunnels that match are established, RPCs will fail with
// "Unavailable" errors. If no affinity key function was provided when the
// handler was created, the only key available will be the nil interface.
//
// The returned channel will use a round-robin strategy to select from matching
// reverse tunnels for any given RPC.
//
// This method panics if the handler was created with an option to disallow the
// use of reverse tunnels.
func (s *TunnelServiceHandler) KeyAsChannel(key interface{}) grpc.ClientConnInterface {
	if s.noReverseTunnels {
		panic("reverse tunnels not supported")
	}
	return multiChannel(func() grpc.ClientConnInterface {
		return s.pickKey(key)
	})
}

func (s *TunnelServiceHandler) pickKey(key interface{}) grpc.ClientConnInterface {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.reverseByKey[key].pick()
}

type multiChannel func() grpc.ClientConnInterface

func (c multiChannel) Invoke(ctx context.Context, methodName string, req, resp interface{}, opts ...grpc.CallOption) error {
	ch := c()
	if ch == nil {
		return status.Errorf(codes.Unavailable, "no channels ready")
	}
	return ch.Invoke(ctx, methodName, req, resp, opts...)
}

func (c multiChannel) NewStream(ctx context.Context, desc *grpc.StreamDesc, methodName string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	ch := c()
	if ch == nil {
		return nil, status.Errorf(codes.Unavailable, "no channels ready")
	}
	return ch.NewStream(ctx, desc, methodName, opts...)
}
