package grpctunnel

import (
	"context"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/fullstorydev/grpchan"
	"github.com/fullstorydev/grpchan/grpchantesting"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func TestTunnelServer(t *testing.T) {
	// Basic tests of the tunnel service as a gRPC channel

	var svr grpchantesting.TestServer

	ready := make(chan struct{})
	ts := TunnelServer{
		OnReverseTunnelConnect: func(*ReverseTunnelChannel) {
			// don't block; just make sure there's something in the channel
			select {
			case ready <- struct{}{}:
			default:
			}
		},
	}
	grpchantesting.RegisterTestServiceServer(&ts, &svr)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	gs := grpc.NewServer()
	RegisterTunnelServiceServer(gs, &ts)
	go func() {
		if err := gs.Serve(l); err != nil {
			t.Logf("error from grpc server: %v", err)
		}
	}()
	defer gs.Stop()

	cc, err := grpc.Dial(l.Addr().String(), grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer func() {
		_ = cc.Close()
	}()

	cli := NewTunnelServiceClient(cc)

	t.Run("forward", func(t *testing.T) {
		checkForGoroutineLeak(t, func() {
			tunnel, err := cli.OpenTunnel(context.Background())
			if err != nil {
				t.Fatalf("failed to open tunnel: %v", err)
			}

			ch := NewChannel(tunnel)
			defer ch.Close()

			grpchantesting.RunChannelTestCases(t, ch, true)
		})
	})

	t.Run("reverse", func(t *testing.T) {
		checkForGoroutineLeak(t, func() {
			ctx, cancel := context.WithCancel(context.Background())
			tunnel, err := cli.OpenReverseTunnel(ctx)
			if err != nil {
				t.Fatalf("failed to open reverse tunnel: %v", err)
			}

			// client now acts as the server
			handlerMap := grpchan.HandlerMap{}
			grpchantesting.RegisterTestServiceServer(handlerMap, &svr)
			errs := make(chan error)
			go func() {
				errs <- ServeReverseTunnel(tunnel, handlerMap)
			}()

			defer func() {
				// TODO: using CloseSend to stop the server can cause errors where "server" (actually
				//  running in the client) tries to send cancellations, but that results in internal
				//  error for whole tunnel RPC due to trying to call SendMsg on a stream after
				//  CloseSend is called.
				//  The right way to do this would be to expose a stop or graceful shutdown mechanism
				//  from the reverse server, which is what can call CloseSend but also prevent any
				//  subsequent attempts to SendMsg.
				//if err := tunnel.CloseSend(); err != nil {
				//	t.Logf("error half-closing ServeReverseTunnel stream: %v", err)
				//}

				// Shutdown via cancelling the context used to create the stream.
				cancel()

				err := <-errs
				// we expect an error related to the above cancellation
				stat, isStatusError := status.FromError(err)
				if err != nil && err != context.Canceled && (!isStatusError || stat.Code() != codes.Canceled) {
					t.Errorf("ServeReverseTunnel returned error: %v", err)
				}
			}()

			// make sure server has registered client, so we can issue RPCs to it
			<-ready
			ch := ts.AsChannel()

			grpchantesting.RunChannelTestCases(t, ch, true)
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
