package grpctunnel

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/fullstorydev/grpchan/grpchantesting"
	"github.com/fullstorydev/grpchan/inprocgrpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/jhump/grpctunnel/tunnelpb"
)

func TestEstablishmentTimeout_Channel(t *testing.T) {
	testCases := []struct {
		name string
		// if true, the server sends headers but never sends settings;
		// otherwise, it never even sends headers (like v0.1)
		sendHeaders   bool
		opts          []TunnelOption
		expectTimeout time.Duration
	}{
		{
			name:          "default",
			expectTimeout: defaultEstablishmentTimeout,
		},
		{
			name:          "custom",
			opts:          []TunnelOption{WithEstablishmentTimeout(time.Second)},
			expectTimeout: time.Second,
		},
		{
			name:          "no-settings",
			sendHeaders:   true,
			opts:          []TunnelOption{WithEstablishmentTimeout(time.Second)},
			expectTimeout: time.Second,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var inproc inprocgrpc.Channel
				tunnelpb.RegisterTunnelServiceServer(&inproc, unresponsiveTunnelServer{sendHeaders: testCase.sendHeaders})
				start := time.Now()
				_, err := NewChannel(tunnelpb.NewTunnelServiceClient(&inproc), testCase.opts...).Start(t.Context())
				require.Equal(t, testCase.expectTimeout, time.Since(start))
				require.Equal(t, codes.FailedPrecondition, status.Code(err))
				require.ErrorContains(t, err, "timed out after "+testCase.expectTimeout.String()+
					" waiting to establish tunnel with server")
			})
		})
	}
}

func TestEstablishmentTimeout_NoLimit(t *testing.T) {
	for _, timeout := range []time.Duration{0, -1} {
		t.Run(timeout.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var inproc inprocgrpc.Channel
				tunnelpb.RegisterTunnelServiceServer(&inproc, unresponsiveTunnelServer{})
				ctx, cancel := context.WithCancel(t.Context())
				startDone := make(chan error, 1)
				go func() {
					_, err := NewChannel(tunnelpb.NewTunnelServiceClient(&inproc), WithEstablishmentTimeout(timeout)).Start(ctx)
					startDone <- err
				}()
				time.Sleep(time.Hour)
				synctest.Wait()
				select {
				case err := <-startDone:
					t.Fatalf("Start returned before context was cancelled: %v", err)
				default:
				}
				cancel()
				// The in-process channel reports cancellation as a plain context
				// error rather than as a status with a Canceled code.
				err := <-startDone
				require.True(t, errors.Is(err, context.Canceled) || status.Code(err) == codes.Canceled,
					"expected cancellation error, got %v", err)
			})
		})
	}
}

func TestEstablishmentTimeout_ReverseTunnelServer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var inproc inprocgrpc.Channel
		tunnelpb.RegisterTunnelServiceServer(&inproc, unresponsiveTunnelServer{})
		revSvr := NewReverseTunnelServer(tunnelpb.NewTunnelServiceClient(&inproc), WithEstablishmentTimeout(time.Second))
		start := time.Now()
		started, err := revSvr.Serve(t.Context())
		require.Equal(t, time.Second, time.Since(start))
		require.False(t, started)
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
		require.ErrorContains(t, err, "timed out after 1s waiting to establish tunnel with tunnel client (network server)")
	})
}

func TestEstablishmentTimeout_DoesNotAffectEstablishedTunnel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var svr grpchantesting.TestServer
		tunnelCli, ts := setupInProcessServer(&svr)
		opt := WithEstablishmentTimeout(time.Second)

		ch, err := NewChannel(tunnelCli, opt).Start(t.Context())
		require.NoError(t, err)
		defer ch.Close()

		revSvr := NewReverseTunnelServer(tunnelCli, opt)
		grpchantesting.RegisterTestServiceServer(revSvr, &svr)
		serveDone := make(chan struct{})
		go func() {
			defer close(serveDone)
			started, err := revSvr.Serve(t.Context())
			require.True(t, started)
			require.NoError(t, err)
		}()
		defer func() {
			revSvr.Stop()
			<-serveDone
		}()
		require.NoError(t, ts.AsChannel().WaitForReady(t.Context()))

		// Both tunnels should still work well after the timeout.
		time.Sleep(time.Minute)
		_, err = grpchantesting.NewTestServiceClient(ch).Unary(t.Context(), &grpchantesting.Message{})
		require.NoError(t, err)
		_, err = grpchantesting.NewTestServiceClient(ts.AsChannel()).Unary(t.Context(), &grpchantesting.Message{})
		require.NoError(t, err)
	})
}

func TestEstablishmentTimeout_TunnelServiceHandler(t *testing.T) {
	// For reverse tunnels, the handler is the tunnel client, so it waits for
	// settings from the tunnel server (the network client).
	testCases := []struct {
		name          string
		timeout       time.Duration
		expectTimeout time.Duration
	}{
		{
			name:          "default",
			expectTimeout: defaultEstablishmentTimeout,
		},
		{
			name:          "custom",
			timeout:       time.Second,
			expectTimeout: time.Second,
		},
		{
			name:    "no-limit",
			timeout: -1,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var opened atomic.Int32
				ts := NewTunnelServiceHandler(TunnelServiceHandlerOptions{
					EstablishmentTimeout: testCase.timeout,
					OnReverseTunnelOpen:  func(TunnelChannel) { opened.Add(1) },
				})
				var inproc inprocgrpc.Channel
				tunnelpb.RegisterTunnelServiceServer(&inproc, ts.Service())
				// Like a ReverseTunnelServer that negotiates but never sends settings.
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				ctx = metadata.AppendToOutgoingContext(ctx, grpctunnelNegotiateKey, grpctunnelNegotiateVal)
				stream, err := tunnelpb.NewTunnelServiceClient(&inproc).OpenReverseTunnel(ctx)
				require.NoError(t, err)
				start := time.Now()
				recvDone := make(chan error, 1)
				go func() {
					_, err := stream.Recv()
					recvDone <- err
				}()

				if testCase.expectTimeout == 0 {
					time.Sleep(time.Hour)
					synctest.Wait()
					select {
					case err := <-recvDone:
						t.Fatalf("tunnel ended before client hung up: %v", err)
					default:
					}
					cancel()
					<-recvDone
					return
				}

				err = <-recvDone
				require.Equal(t, testCase.expectTimeout, time.Since(start))
				require.Equal(t, codes.FailedPrecondition, status.Code(err))
				require.ErrorContains(t, err, "timed out after "+testCase.expectTimeout.String()+
					" waiting to establish tunnel with tunnel server (network client)")
				require.Zero(t, opened.Load(), "OnReverseTunnelOpen should not be called for tunnel that was never established")
			})
		})
	}
}

func TestTunnelStreamContextCancelledInBubble(t *testing.T) {
	// The in-process channel only cancels a stream's context in a finalizer,
	// which runs outside of any synctest bubble. So if we didn't cancel the
	// context of the tunnel's stream ourselves when the tunnel is done, then
	// garbage collection could crash the test, when the finalizer closes a
	// channel that belongs to the bubble.
	realDelay := make(chan struct{})
	time.AfterFunc(200*time.Millisecond, func() { close(realDelay) })
	synctest.Test(t, func(t *testing.T) {
		var svr grpchantesting.TestServer
		tunnelCli, _ := setupInProcessServer(&svr)
		for _, opt := range []TunnelOption{WithEstablishmentTimeout(time.Second), WithEstablishmentTimeout(0)} {
			ch, err := NewChannel(tunnelCli, opt).Start(t.Context())
			require.NoError(t, err)
			ch.Close()
			<-ch.Done()
		}
		synctest.Wait()
		// Collect the tunnel's stream and give its finalizer a chance to run
		// while this bubble is still active. (Receiving from a channel that was
		// created outside the bubble waits in real time.)
		runtime.GC()
		runtime.GC()
		<-realDelay
	})
}

// unresponsiveTunnelServer is a tunnel server that never finishes establishing
// tunnels. If sendHeaders is false, it never even sends response headers (like
// v0.1). Otherwise, it sends headers but never sends settings.
type unresponsiveTunnelServer struct {
	tunnelpb.UnimplementedTunnelServiceServer
	sendHeaders bool
}

func (s unresponsiveTunnelServer) OpenTunnel(stream tunnelpb.TunnelService_OpenTunnelServer) error {
	if s.sendHeaders {
		if err := stream.SendHeader(metadata.Pairs(grpctunnelNegotiateKey, grpctunnelNegotiateVal)); err != nil {
			return err
		}
	}
	<-stream.Context().Done()
	return stream.Context().Err()
}

func (s unresponsiveTunnelServer) OpenReverseTunnel(stream tunnelpb.TunnelService_OpenReverseTunnelServer) error {
	<-stream.Context().Done()
	return stream.Context().Err()
}
