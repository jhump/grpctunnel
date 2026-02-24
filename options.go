package grpctunnel

// TunnelOption is an option for configuring the behavior of
// a tunnel client or tunnel server.
type TunnelOption interface {
	apply(*tunnelOpts)
}

func WithInitialWindowSize(size uint32) TunnelOption {
	return tunnelOptFunc(func(t *tunnelOpts) {
		t.initialWindowSize = size
	})
}

func WithMaxChunkSize(size uint32) TunnelOption {
	return tunnelOptFunc(func(t *tunnelOpts) {
		t.maxChunkSize = size
	})
}

type tunnelOpts struct {
	initialWindowSize uint32
	maxChunkSize      uint32

	// TODO: Option for minimum update size, so receiver can choose to batch
	//       window updates, which can help throughput by eliminating some of
	//       the bandwidth used for flow control messages.
}

func initOptions(t *tunnelOpts, opts []TunnelOption) {
	for _, opt := range opts {
		opt.apply(t)
	}
	if t.initialWindowSize == 0 {
		t.initialWindowSize = defaultInitialWindowSize
	}
	if t.maxChunkSize == 0 {
		t.maxChunkSize = defaultChunkMax
	}
}

type tunnelOptFunc func(*tunnelOpts)

func (t tunnelOptFunc) apply(opts *tunnelOpts) {
	t(opts)
}
