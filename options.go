package grpctunnel

// TunnelOption is an option for configuring the behavior of
// a tunnel client or tunnel server.
type TunnelOption interface {
	apply(*tunnelOpts)
}

// WithInitialWindowSize configures the initial flow control window size. If
// this option is not used or if this option is used to set the value to zero,
// a default value of 64k will be used. Increasing this may increase total
// throughput at the cost of more memory usage.
func WithInitialWindowSize(size uint32) TunnelOption {
	return tunnelOptFunc(func(t *tunnelOpts) {
		t.initialWindowSize = size
	})
}

// WithMaxChunkSize configured the maximum size of a single chunk of data to send.
// This will be clamped to the peer's initial flow control window size if set to a
// larger value. If this option is not used or if this option is used to set the
// value to zero, a default max chunk size of 16k will be used. Increasing this can
// allow larger messages to be sent more quickly (fewer chunks, fewer flow control
// messages) but at the potential cost of fairness, in the event that multiple
// streams are trying to concurrently send large messages.
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
