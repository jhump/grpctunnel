package internal

import (
	"context"
	"net"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// BlockingDial dials the given address and returns the resulting gRPC client conn.
// It blocks for the client to become ready. If the given context finishes before the
// client becomes ready, it returns the most recent error returned by underlying
// network dial operations. If no such error has been returned, it will return the
// context error.
func BlockingDial(ctx context.Context, addr string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	dialingDone := make(chan struct{})
	defer close(dialingDone)
	dialErrors := make(chan error, 1)
	cc, err := grpc.NewClient(addr, append(opts,
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			// Copied from grpc-go's internal.NetDialerWithTCPKeepalive function, which
			// is the default dial behavior.
			conn, err := (&net.Dialer{
				// Setting a negative value here prevents the Go stdlib from overriding
				// the values of TCP keepalive time and interval. It also prevents the
				// Go stdlib from enabling TCP keepalives by default.
				KeepAlive: time.Duration(-1),
				// This method is called after the underlying network socket is created,
				// but before dialing the socket (or calling its connect() method). The
				// combination of unconditionally enabling TCP keepalives here, and
				// disabling the overriding of TCP keepalive parameters by setting the
				// KeepAlive field to a negative value above, results in OS defaults for
				// the TCP keepalive interval and time parameters.
				Control: func(_, _ string, c syscall.RawConn) error {
					return c.Control(func(fd uintptr) {
						_ = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_KEEPALIVE, 1)
					})
				},
			}).DialContext(ctx, "tcp", addr)
			if err != nil {
				select {
				case <-dialingDone:
				case dialErrors <- err:
				}
				if !isTemporary(err) {
					cancel()
				}
			}
			return conn, err
		}))...,
	)
	if err != nil {
		return nil, err
	}
	var dialErr error
	var mu sync.Mutex
	go func() {
		for {
			select {
			case <-dialingDone:
				return
			case <-ctx.Done():
				return
			case err := <-dialErrors:
				mu.Lock()
				dialErr = err
				mu.Unlock()
			}
		}
	}()
	cc.Connect()
	for {
		connState := cc.GetState()
		if connState == connectivity.Ready {
			return cc, nil
		}
		if !cc.WaitForStateChange(ctx, connState) {
			mu.Lock()
			err := dialErr
			mu.Unlock()
			if err != nil {
				return nil, err
			}
			// Don't have a dialErr? Return the context error..
			return nil, ctx.Err()
		}
	}
}

// copied from grpc-go
func isTemporary(err error) bool {
	switch err := err.(type) {
	case interface {
		Temporary() bool
	}:
		return err.Temporary()
	case interface {
		Timeout() bool
	}:
		// Timeouts may be resolved upon retry, and are thus treated as
		// temporary.
		return err.Timeout()
	}
	return true
}
