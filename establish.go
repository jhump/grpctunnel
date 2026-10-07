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
//
// The context is also cancelled when release is called, which must be done
// once the RPC is finished. Doing this ourselves, instead of relying on the
// transport to cancel the stream's context when the RPC finishes, ensures it
// happens promptly with any transport. (For example, the in-process channel in
// grpchan only cancels a stream's context when the stream is garbage
// collected.)
func startEstablishment(ctx context.Context, timeout time.Duration, peer string) (context.Context, *establishment) {
	ctx, cancel := context.WithCancelCause(ctx)
	est := &establishment{cancel: cancel}
	if timeout > 0 {
		est.err = establishmentTimeoutError(timeout, peer)
		est.timer = time.AfterFunc(timeout, func() { cancel(est.err) })
	}
	return ctx, est
}

// establishmentTimeoutError returns the error for a tunnel that could not be
// established with the given peer within the given timeout.
func establishmentTimeoutError(timeout time.Duration, peer string) error {
	return status.Errorf(codes.FailedPrecondition,
		"timed out after %v waiting to establish tunnel with %s; it may be unresponsive or using an unsupported version of grpctunnel",
		timeout, peer)
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
	e.cancel(nil)
}
