package grpctunnel

import (
	"context"
	"google.golang.org/grpc/metadata"
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
	onReverseTunnelConnect    func(TunnelChannel)
	onReverseTunnelDisconnect func(TunnelChannel)
	affinityKey               func(TunnelChannel) any

	stopping atomic.Bool
	reverse  *reverseChannels

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
	OnReverseTunnelOpen func(TunnelChannel)
	// If reverse tunnels are allowed, this callback may be configured to
	// receive information when reverse tunnels are torn down.
	OnReverseTunnelClose func(TunnelChannel)
	// Optional function that accepts a reverse tunnel and returns an affinity
	// key. The affinity key values can be used to look up outbound channels,
	// for targeting calls to particular clients or groups of clients.
	//
	// The given TunnelChannel has a context which can be used to query for the
	// identity of the remote peer. Functions like peer.FromContext will return
	// the peer that opened the reverse tunnel, and metadata.FromIncomingContext
	// will return the request headers used to open the reverse tunnel. If any
	// server interceptors ran when the tunnel was opened, then any values they
	// store in the context is also available.
	AffinityKey func(TunnelChannel) any
}

// NewTunnelServiceHandler creates a new TunnelServiceHandler. The options are
// used to configure behavior for reverse tunnels. The returned handler is also
// a [grpc.ServiceRegistrar], to register the services that will be available
// for forward tunnels.
//
// The handler's Service method can be used to actually register the handler
// with a *grpc.Server (or other grpc.ServiceRegistrar).
func NewTunnelServiceHandler(options TunnelServiceHandlerOptions) *TunnelServiceHandler {
	return &TunnelServiceHandler{
		handlers:                  grpchan.HandlerMap{},
		noReverseTunnels:          options.NoReverseTunnels,
		onReverseTunnelConnect:    options.OnReverseTunnelOpen,
		onReverseTunnelDisconnect: options.OnReverseTunnelClose,
		affinityKey:               options.AffinityKey,
		reverse:                   newReverseChannels(),
		reverseByKey:              map[interface{}]*reverseChannels{},
	}
}

var _ grpc.ServiceRegistrar = (*TunnelServiceHandler)(nil)

// RegisterService implements the grpc.ServiceRegistrar interface. This allows the
// handler to be passed to generated registration functions, so service
// implementations can be registered with the handler.
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
	md, _ := metadata.FromIncomingContext(stream.Context())
	return serveTunnel(stream, md, s.handlers, s.stopping.Load)
}

func (s *TunnelServiceHandler) openReverseTunnel(stream tunnelpb.TunnelService_OpenReverseTunnelServer) error {
	if s.noReverseTunnels {
		return status.Error(codes.Unimplemented, "reverse tunnels not supported")
	}

	// Immediately send headers instead of waiting for first RPC to send a message.
	// This gives any server interceptors a chance to run and potentially to send
	// auth credentials in response headers (since the client will need a way to
	// authenticate the server, since roles are reversed with reverse tunnels).
	_ = stream.SendHeader(nil)

	ch := newReverseChannel(stream, s.unregister)
	defer ch.Close()

	var key interface{}
	if s.affinityKey != nil {
		key = s.affinityKey(ch)
	}

	s.reverse.add(ch, key)
	defer s.reverse.remove(ch)

	rc := s.reverseChannelsForKey(key)
	rc.add(ch, key)
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

func (s *TunnelServiceHandler) unregister(ch *tunnelChannel) {
	k, ok := s.reverse.remove(ch)
	if !ok {
		// already removed
		return
	}

	s.mu.Lock()
	rc := s.reverseByKey[k]
	s.mu.Unlock()
	if rc != nil {
		rc.remove(ch)
	}
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
	avail chan struct{}
	chans []reverseChannelEntry
	idx   int
}

type reverseChannelEntry struct {
	ch  *tunnelChannel
	key any
}

func newReverseChannels() *reverseChannels {
	return &reverseChannels{
		avail: make(chan struct{}),
	}
}

func (c *reverseChannels) allChans() []TunnelChannel {
	c.mu.Lock()
	defer c.mu.Unlock()

	cp := make([]TunnelChannel, len(c.chans))
	for i := range c.chans {
		cp[i] = c.chans[i].ch
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
	return c.chans[c.idx].ch
}

func (c *reverseChannels) add(ch *tunnelChannel, key any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.chans = append(c.chans, reverseChannelEntry{ch: ch, key: key})
	if len(c.chans) == 1 {
		// first channel means we are now ready
		close(c.avail)
	}
}

func (c *reverseChannels) remove(ch *tunnelChannel) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.chans {
		if c.chans[i].ch == ch {
			key := c.chans[i].key
			c.chans = append(c.chans[:i], c.chans[i+1:]...)
			if len(c.chans) == 0 {
				// need a new channel for waiting on ready
				c.avail = make(chan struct{})
			}
			return key, true
		}
	}
	return nil, false
}

func (c *reverseChannels) ready() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.chans) > 0
}

func (c *reverseChannels) waitForReady(ctx context.Context) error {
	c.mu.Lock()
	avail := c.avail
	c.mu.Unlock()

	select {
	case <-avail:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// AllReverseTunnels returns the set of all currently active reverse tunnels.
func (s *TunnelServiceHandler) AllReverseTunnels() []TunnelChannel {
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
func (s *TunnelServiceHandler) AsChannel() ReverseClientConnInterface {
	if s.noReverseTunnels {
		panic("reverse tunnels not supported")
	}
	return multiChannel{
		pick:         s.reverse.pick,
		ready:        s.reverse.ready,
		waitForReady: s.reverse.waitForReady,
	}
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
func (s *TunnelServiceHandler) KeyAsChannel(key interface{}) ReverseClientConnInterface {
	if s.noReverseTunnels {
		panic("reverse tunnels not supported")
	}
	return multiChannel{
		pick: func() grpc.ClientConnInterface {
			return s.pickKey(key)
		},
		ready: func() bool {
			return s.keyIsReady(key)
		},
		waitForReady: func(ctx context.Context) error {
			return s.waitForKeyReady(ctx, key)
		},
	}
}

func (s *TunnelServiceHandler) pickKey(key interface{}) grpc.ClientConnInterface {
	s.mu.RLock()
	rc := s.reverseByKey[key]
	s.mu.RUnlock()

	if rc == nil {
		return nil
	}
	return rc.pick()
}

func (s *TunnelServiceHandler) keyIsReady(key interface{}) bool {
	s.mu.RLock()
	rc := s.reverseByKey[key]
	s.mu.RUnlock()

	if rc == nil {
		return false
	}
	return rc.ready()
}

func (s *TunnelServiceHandler) waitForKeyReady(ctx context.Context, key interface{}) error {
	rc := s.reverseChannelsForKey(key)
	return rc.waitForReady(ctx)
}

func (s *TunnelServiceHandler) reverseChannelsForKey(key any) *reverseChannels {
	s.mu.Lock()
	defer s.mu.Unlock()

	rc := s.reverseByKey[key]
	if rc == nil {
		rc = newReverseChannels()
		s.reverseByKey[key] = rc
	}
	return rc
}

// ReverseClientConnInterface is an RPC channel that reports if there are any
// active reverse tunnels. If there are no available tunnels, the channel will
// not be ready. When tunnels are ready, RPCs will be balanced across all such
// tunnels in a round-robin fashion.
type ReverseClientConnInterface interface {
	grpc.ClientConnInterface
	// Ready returns true if this channel is backed by at least one reverse
	// tunnel. Returns false if there are no active, matching reverse tunnels.
	Ready() bool
	// WaitForReady blocks until either this channel is ready or the given
	// context is cancelled or times out. If the latter, the context error is
	// returned. Otherwise, nil is returned. If the channel is immediately
	// ready, this will not block and will immediately return nil.
	WaitForReady(context.Context) error
}

type multiChannel struct {
	pick         func() grpc.ClientConnInterface
	ready        func() bool
	waitForReady func(context.Context) error
}

func (c multiChannel) Invoke(ctx context.Context, methodName string, req, resp interface{}, opts ...grpc.CallOption) error {
	ch := c.pick()
	if ch == nil {
		return status.Errorf(codes.Unavailable, "no channels ready")
	}
	return ch.Invoke(ctx, methodName, req, resp, opts...)
}

func (c multiChannel) NewStream(ctx context.Context, desc *grpc.StreamDesc, methodName string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	ch := c.pick()
	if ch == nil {
		return nil, status.Errorf(codes.Unavailable, "no channels ready")
	}
	return ch.NewStream(ctx, desc, methodName, opts...)
}

func (c multiChannel) Ready() bool {
	return c.ready()
}

func (c multiChannel) WaitForReady(ctx context.Context) error {
	return c.waitForReady(ctx)
}
