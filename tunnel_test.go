package grpctunnel

import (
	"context"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/fullstorydev/grpchan/grpchantesting"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/jhump/grpctunnel/tunnelpb"
)

func TestTunnelServer(t *testing.T) {
	// Basic tests of the tunnel service as a gRPC channel

	var svr grpchantesting.TestServer

	ready := make(chan struct{})
	ts := NewTunnelServiceHandler(TunnelServiceHandlerOptions{
		OnReverseTunnelConnect: func(ReverseTunnelChannel) {
			// don't block; just make sure there's something in the channel
			select {
			case ready <- struct{}{}:
			default:
			}
		},
	})
	grpchantesting.RegisterTestServiceServer(ts, &svr)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	gs := grpc.NewServer()
	tunnelpb.RegisterTunnelServiceServer(gs, ts.Service())
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

	cli := tunnelpb.NewTunnelServiceClient(cc)

	// Make sure any goroutines used by the client and server created above have started. That
	// way, we don't incorrectly think they are leaked goroutines.
	time.Sleep(500 * time.Millisecond)

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
			revSvr := NewReverseTunnelServer(cli)
			grpchantesting.RegisterTestServiceServer(revSvr, &svr)
			go func() {
				if err, _ := revSvr.Serve(context.Background()); err != nil {
					t.Logf("ReverseTunnelServer.Serve returned error: %v", err)
				}
			}()
			defer func() {
				revSvr.Stop()
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
