package grpctunnel

import (
	"context"
	"errors"
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
