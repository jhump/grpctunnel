package grpctunnel

import "github.com/jhump/grpctunnel/tunnelpb"

// TunnelOption is an option for configuring the behavior of
// a tunnel client or tunnel server.
type TunnelOption interface {
	apply(*tunnelOpts)
}

// WithDisableFlowControl returns an option that disables the
// use of flow control, even when the tunnel peer supports it.
//
// NOTE: This should NOT be used in application code. This is
// intended for test code, to verify that the tunnels work
// without flow control, to make sure they can interop correctly
// with older versions of this package, before flow control was
// introduced.
//
// Eventually, older versions that do not use flow control will
// not be supported and this option will be removed.
func WithDisableFlowControl() TunnelOption {
	return tunnelOptFunc(func(opts *tunnelOpts) {
		opts.disableFlowControl = true
	})
}

type tunnelOpts struct {
	disableFlowControl bool
}

func (t *tunnelOpts) supportedRevisions() []tunnelpb.ProtocolRevision {
	if t.disableFlowControl {
		return []tunnelpb.ProtocolRevision{tunnelpb.ProtocolRevision_REVISION_ZERO}
	}
	return []tunnelpb.ProtocolRevision{
		tunnelpb.ProtocolRevision_REVISION_ZERO, tunnelpb.ProtocolRevision_REVISION_ONE,
	}
}

type tunnelOptFunc func(*tunnelOpts)

func (t tunnelOptFunc) apply(opts *tunnelOpts) {
	t(opts)
}
