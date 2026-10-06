package grpctunnel

// TunnelOption is an option for configuring the behavior of
// a tunnel client or tunnel server.
type TunnelOption interface {
	apply(*tunnelOpts)
}

// WithInitialWindowSize configures the initial flow control window size for
// receiving data. (The peer sets the initial window size for sending data.) If
// this option is not used or if this option is used to set the value to zero,
// a default value of 64k will be used. Values less than 1k will be increased to
// 1k. Increasing this may increase total throughput at the cost of more memory
// usage.
func WithInitialWindowSize(size uint32) TunnelOption {
	return tunnelOptFunc(func(t *tunnelOpts) {
		t.initialWindowSize = size
	})
}

// WithMaxChunkSize configures the maximum size of a single chunk of data to send.
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

// WithMinWindowUpdateSize sets the minimum size for a flow control window update
// message. This will be clamped to the initial window size if set to a larger value.
// When receiving data, a window update will not be sent unless there is at least
// this amount outstanding (i.e. this many bytes to acknowledge). When unset or zero,
// this will default to 16k. When set to one, there is effectively no minimum, and
// an update window message will be sent for every chunk received, regardless of how
// small. (When RPC traffic consists of a lot of small messages, this can result in
// high bandwidth overhead for flow control management.) A larger value means fewer
// window update messages (and thus less overhead for flow control management), but
// too large a value, especially combined with RPC traffic that uses large messages,
// could mean an increase in latency while the sender waits for the large update
// window message before it can send more data.
func WithMinWindowUpdateSize(size uint32) TunnelOption {
	return tunnelOptFunc(func(t *tunnelOpts) {
		t.minWindowUpdateSize = size
	})
}

type tunnelOpts struct {
	initialWindowSize   uint32
	maxChunkSize        uint32
	minWindowUpdateSize uint32
}

func initOptions(t *tunnelOpts, opts []TunnelOption) {
	for _, opt := range opts {
		opt.apply(t)
	}
	if t.initialWindowSize == 0 {
		t.initialWindowSize = defaultInitialWindowSize
	} else if t.initialWindowSize < minInitialWindowSize {
		t.initialWindowSize = minInitialWindowSize
	}
	if t.maxChunkSize == 0 {
		t.maxChunkSize = defaultChunkMax
	}
	if t.minWindowUpdateSize == 0 {
		t.minWindowUpdateSize = defaultUpdateMin
	}
}

type tunnelOptFunc func(*tunnelOpts)

func (t tunnelOptFunc) apply(opts *tunnelOpts) {
	t(opts)
}
