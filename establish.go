package grpctunnel

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// establishment limits how long it can take to establish a tunnel. See
// WithEstablishmentTimeout.
type establishment struct {
	timer  *time.Timer
	cancel context.CancelCauseFunc
	err    error
}

// startEstablishment returns a context to use for the RPC that opens a tunnel.
// If the tunnel isn't established within the given timeout, the context is
// cancelled. If the timeout is not positive, there is no limit. The given peer
// describes the other end of the tunnel, for use in error messages.
func startEstablishment(ctx context.Context, timeout time.Duration, peer string) (context.Context, *establishment) {
	if timeout <= 0 {
		return ctx, &establishment{}
	}
	ctx, cancel := context.WithCancelCause(ctx)
	est := &establishment{
		cancel: cancel,
		err: status.Errorf(codes.FailedPrecondition,
			"timed out after %v waiting to establish tunnel with %s; it may be unresponsive or using an unsupported version of grpctunnel",
			timeout, peer),
	}
	est.timer = time.AfterFunc(timeout, func() { cancel(est.err) })
	return ctx, est
}

// done must be called exactly once: when the tunnel is established, or when
// establishing it fails with the given error. If the timeout elapsed first,
// this returns the timeout error instead.
func (e *establishment) done(err error) error {
	if e.timer != nil && !e.timer.Stop() {
		return e.err
	}
	return err
}

// release releases resources associated with the context returned by
// startEstablishment. It must be called once the RPC that opens the tunnel
// has finished.
func (e *establishment) release() {
	if e.cancel != nil {
		e.cancel(nil)
	}
}
