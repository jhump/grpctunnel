package grpctunnel

import (
	"context"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/fullstorydev/grpchan/grpchantesting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/jhump/grpctunnel/tunnelpb"
)

func TestTunnelServer(t *testing.T) {
	// Basic tests of the tunnel service as a gRPC channel

	var svr grpchantesting.TestServer

	ts := NewTunnelServiceHandler(TunnelServiceHandlerOptions{
		AffinityKey: func(t ReverseTunnelChannel) any {
			vals := t.RequestHeaders().Get("nesting-level")
			if len(vals) == 0 {
				return ""
			}
			return vals[0]
		},
	})
	grpchantesting.RegisterTestServiceServer(ts, &svr)
	// recursive: tunnels can be run on top of tunnels
	// (not realistic, but fun exercise to verify soundness of protocol)
	tunnelpb.RegisterTunnelServiceServer(ts, ts.Service())

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "failed to listen")
	gs := grpc.NewServer()
	tunnelpb.RegisterTunnelServiceServer(gs, ts.Service())
	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		assert.NoError(t, gs.Serve(l), "error from grpc server")
	}()
	defer func() {
		gs.Stop()
		<-serveDone
	}()

	cc, err := grpc.Dial(l.Addr().String(), grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "failed top create client")
	defer func() {
		err := cc.Close()
		require.NoError(t, err, "failed to close client conn")
	}()

	cli := tunnelpb.NewTunnelServiceClient(cc)

	// Make sure any goroutines used by the client and server created above have started. That
	// way, we don't incorrectly think they are leaked goroutines.
	time.Sleep(500 * time.Millisecond)

	runTests(context.Background(), t, false, cli, ts, &svr)
}

func runTests(ctx context.Context, t *testing.T, nested bool, cli tunnelpb.TunnelServiceClient, ts *TunnelServiceHandler, testSvr *grpchantesting.TestServer) {
	if nested {
		ctx = metadata.AppendToOutgoingContext(ctx, "nesting-level", "1")
	}

	prefix := ""
	if nested {
		prefix = "nested-"
	}
	t.Run(prefix+"forward", func(t *testing.T) {
		checkForGoroutineLeak(t, func() {
			tunnel, err := cli.OpenTunnel(ctx)
			require.NoError(t, err, "failed to open tunnel")

			ch := NewChannel(tunnel)
			defer func() {
				ch.Close()
				<-ch.Done()
				assert.NoError(t, ch.Err(), "channel ended with error")
			}()

			grpchantesting.RunChannelTestCases(t, ch, true)

			if !nested {
				// nested/recursive test
				runTests(ch.Context(), t, true, tunnelpb.NewTunnelServiceClient(ch), ts, testSvr)
			}
		})
	})

	t.Run(prefix+"reverse", func(t *testing.T) {
		checkForGoroutineLeak(t, func() {
			revSvr := NewReverseTunnelServer(cli)
			if !nested {
				// we need this to run the nested/recursive tunnel test
				tunnelpb.RegisterTunnelServiceServer(revSvr, ts.Service())
			}
			grpchantesting.RegisterTestServiceServer(revSvr, testSvr)
			serveDone := make(chan struct{})
			go func() {
				defer close(serveDone)
				err, ok := revSvr.Serve(ctx)
				assert.True(t, ok, "ReverseTunnelServer.Serve returned false")
				assert.NoError(t, err, "ReverseTunnelServer.Serve returned error")
			}()
			defer func() {
				revSvr.Stop()
				<-serveDone
			}()

			// make sure server has registered client, so we can issue RPCs to it
			var ch ReverseClientConnInterface
			if nested {
				ch = ts.KeyAsChannel("1")
			} else {
				ch = ts.AsChannel()
			}
			timedCtx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			err := ch.WaitForReady(timedCtx)
			require.NoError(t, err, "reverse channel never became ready")

			grpchantesting.RunChannelTestCases(t, ch, true)

			if !nested {
				// nested/recursive test
				runTests(ctx, t, true, tunnelpb.NewTunnelServiceClient(ch), ts, testSvr)
			}

			for i, rt := range ts.AllReverseTunnels() {
				assert.NoError(t, rt.Err(), "reverse tunnel channel %d ended with error", i)
			}
		})
	})
}

func checkForGoroutineLeak(t *testing.T, fn func()) {
	before := runtime.NumGoroutine()

	fn()

	// check for goroutine leaks
	deadline := time.Now().Add(time.Second * 5)
	after := 0
	for deadline.After(time.Now()) {
		after = runtime.NumGoroutine()
		if after <= before {
			// number of goroutines returned to previous level: no leak!
			return
		}
		time.Sleep(time.Millisecond * 50)
	}
	buf := make([]byte, 1024*1024)
	n := runtime.Stack(buf, true)
	t.Errorf("%d goroutines leaked:\n%s", after-before, string(buf[:n]))
}

// TODO: also need more tests around channel lifecycle, and ensuring it
// properly respects things like context cancellations, etc

// TODO: also need some concurrency checks, to make sure the channel works
// as expected, and race detector finds no bugs, when used from many
// goroutines at once
