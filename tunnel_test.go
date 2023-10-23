package grpctunnel

import (
	"bytes"
	"context"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fullstorydev/grpchan/grpchantesting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/jhump/grpctunnel/tunnelpb"
)

func TestTunnelServiceHandler(t *testing.T) {
	// Basic tests of the tunnel service as a gRPC channel

	var svr grpchantesting.TestServer
	flowControlCases := []struct {
		name     string
		disabled bool
	}{
		{
			name:     "with-flow-control",
			disabled: false,
		},
		{
			name:     "without-flow-control",
			disabled: true,
		},
	}
	for _, flowControlCase := range flowControlCases {
		t.Run(flowControlCase.name, func(t *testing.T) {
			cli, ts := setupServer(t, &svr, flowControlCase.disabled)
			runTests(context.Background(), t, modeRunNested, cli, ts, &svr,
				func(_ context.Context, t *testing.T, ch grpc.ClientConnInterface) {
					grpchantesting.RunChannelTestCases(t, ch, true)
				})
		})
	}
}

func TestTunnelServiceHandler_Deadlocks(t *testing.T) {
	var svr grpchantesting.TestServer
	cli, ts := setupServer(t, &svr, false)

	runTests(context.Background(), t, modeRunNested, cli, ts, &svr,
		func(ctx context.Context, t *testing.T, ch grpc.ClientConnInterface) {
			runDeadlockTests(ctx, t, ch)
		})
}

type nestingMode int

const (
	modeDoNotRunNested = nestingMode(iota)
	modeRunNested
	modeIsNested
)

func runTests(
	ctx context.Context,
	t *testing.T,
	mode nestingMode,
	cl tunnelpb.TunnelServiceClient,
	ts *TunnelServiceHandler,
	testSvr *grpchantesting.TestServer,
	testFunc func(ctx context.Context, t *testing.T, ch grpc.ClientConnInterface),
) {
	prefix := ""
	if mode == modeIsNested {
		prefix = "nested-"
		ctx = metadata.AppendToOutgoingContext(ctx, "nesting-level", "1")
	}

	t.Run(prefix+"forward", func(t *testing.T) {
		checkForGoroutineLeak(t, func() {
			ch, err := NewChannel(cl).Start(ctx)
			require.NoError(t, err, "failed to open tunnel")

			defer func() {
				ch.Close()
				<-ch.Done()
				assert.NoError(t, ch.Err(), "channel ended with error")
			}()

			testFunc(ctx, t, ch)

			if mode == modeRunNested {
				// nested/recursive test
				runTests(ch.Context(), t, modeIsNested,
					tunnelpb.NewTunnelServiceClient(ch),
					ts, testSvr, testFunc,
				)
			}
		})
	})

	t.Run(prefix+"reverse", func(t *testing.T) {
		checkForGoroutineLeak(t, func() {
			revSvr := NewReverseTunnelServer(cl)
			if mode == modeRunNested {
				// we need this to run the nested/recursive tunnel test
				tunnelpb.RegisterTunnelServiceServer(revSvr, ts.Service())
			}
			grpchantesting.RegisterTestServiceServer(revSvr, testSvr)
			serveDone := make(chan struct{})
			go func() {
				defer close(serveDone)
				started, err := revSvr.Serve(ctx)
				assert.True(t, started, "ReverseTunnelServer.Serve returned false")
				assert.NoError(t, err, "ReverseTunnelServer.Serve returned error")
			}()
			defer func() {
				revSvr.Stop()
				<-serveDone
			}()

			// make sure server has registered client, so we can issue RPCs to it
			var ch ReverseClientConnInterface
			if mode == modeIsNested {
				ch = ts.KeyAsChannel("1")
			} else {
				ch = ts.AsChannel()
			}
			timedCtx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			err := ch.WaitForReady(timedCtx)
			require.NoError(t, err, "reverse channel never became ready")

			testFunc(ctx, t, ch)

			if mode == modeRunNested {
				// nested/recursive test
				runTests(
					ctx, t, modeIsNested,
					tunnelpb.NewTunnelServiceClient(ch),
					ts, testSvr, testFunc,
				)
			}

			for i, rt := range ts.AllReverseTunnels() {
				assert.NoError(t, rt.Err(), "reverse tunnel channel %d ended with error", i)
			}
		})
	})
}

func runDeadlockTests(ctx context.Context, t *testing.T, ch grpc.ClientConnInterface) {
	stub := grpchantesting.NewTestServiceClient(ch)
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	slowOneDone := make(chan struct{})
	defer func() {
		cancel()
		<-slowOneDone
	}()
	slowCtx := ctx
	go func() {
		// the slow one
		defer close(slowOneDone)

		stream, err := stub.BidiStream(slowCtx)
		require.NoError(t, err)
		for i := 0; i < 2; i++ {
			err := stream.Send(&grpchantesting.Message{
				DelayMillis: 1000,
				Payload:     bytes.Repeat([]byte{0, 1, 2, 3}, 5_000),
			})
			if err != nil {
				require.Error(t, ctx.Err())
				break
			}
		}
	}()
	time.Sleep(100 * time.Millisecond) // make sure the slow one has had time to issue its RPC

	grp, ctx := errgroup.WithContext(ctx)
	for i := 0; i < 20; i++ {
		grp.Go(func() error {
			// this should proceed just fine, regardless of the slow one
			stream, err := stub.ClientStream(ctx)
			if err != nil {
				return err
			}
			for j := 0; j < 100; j++ {
				err := stream.Send(&grpchantesting.Message{
					Payload: bytes.Repeat([]byte{0, 1, 2, 3}, 10_000),
				})
				if err != nil {
					return err
				}
			}
			_, err = stream.CloseAndRecv()
			return err
		})
	}
	err := grp.Wait()
	require.NoError(t, err)
}

func TestTunnelServiceHandler_Concurrency(t *testing.T) {
	flowControlCases := []struct {
		name     string
		disabled bool
	}{
		{
			name:     "with-flow-control",
			disabled: false,
		},
		{
			name:     "without-flow-control",
			disabled: true,
		},
	}
	for _, flowControlCase := range flowControlCases {
		t.Run(flowControlCase.name, func(t *testing.T) {
			var svr grpchantesting.TestServer
			tunnelCli, ts := setupServer(t, &svr, flowControlCase.disabled)

			forwardCh, err := NewChannel(tunnelCli).Start(context.Background())
			require.NoError(t, err)
			defer func() {
				forwardCh.Close()
				<-forwardCh.Done()
				require.NoError(t, forwardCh.Err())
			}()

			revSvr := NewReverseTunnelServer(tunnelCli)
			grpchantesting.RegisterTestServiceServer(revSvr, &svr)
			serveDone := make(chan struct{})
			go func() {
				defer close(serveDone)
				started, err := revSvr.Serve(context.Background())
				assert.True(t, started, "ReverseTunnelServer.Serve returned false")
				assert.NoError(t, err, "ReverseTunnelServer.Serve returned error")
			}()
			defer func() {
				revSvr.Stop()
				<-serveDone
			}()

			// make sure server has registered client, so we can issue RPCs to it
			reverseCh := ts.AsChannel()
			timedCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err = reverseCh.WaitForReady(timedCtx)
			require.NoError(t, err, "reverse channel never became ready")

			// Make sure any goroutines used by the client and server created above have started. That
			// way, we don't incorrectly think they are leaked goroutines.
			time.Sleep(200 * time.Millisecond)

			testCases := []struct {
				name string
				ch   grpc.ClientConnInterface
			}{
				{
					name: "forward",
					ch:   forwardCh,
				},
				{
					name: "reverse",
					ch:   reverseCh,
				},
			}

			for _, testCase := range testCases {
				cli := grpchantesting.NewTestServiceClient(testCase.ch)
				t.Run(testCase.name, func(t *testing.T) {
					done := make(chan struct{})
					var count int32
					runOneThread := func() {
						for {
							select {
							case <-done:
								return
							default:
							}
							_, err := cli.Unary(context.Background(), &grpchantesting.Message{})
							require.NoError(t, err)
							atomic.AddInt32(&count, 1)
						}
					}

					// Ten goroutines all using the same tunnel, hoping to catch data races or
					// other concurrency-related bugs.
					checkForGoroutineLeak(t, func() {
						var wg sync.WaitGroup
						for i := 0; i < 10; i++ {
							wg.Add(1)
							go func() {
								defer wg.Done()
								runOneThread()
							}()
						}
						// all threads sending concurrent requests for 3 seconds
						time.Sleep(2 * time.Second)
						close(done)
						wg.Wait()
					})

					t.Logf("RPCs sent: %d", atomic.LoadInt32(&count))
				})
			}
		})
	}
}

// TODO: also need more tests around channel lifecycle, and ensuring it
// properly respects things like context cancellations, etc

func setupServer(t *testing.T, svc grpchantesting.TestServiceServer, disableFlowControl bool) (tunnelpb.TunnelServiceClient, *TunnelServiceHandler) {
	ts := NewTunnelServiceHandler(TunnelServiceHandlerOptions{
		AffinityKey: func(t TunnelChannel) any {
			md, _ := metadata.FromIncomingContext(t.Context())
			vals := md.Get("nesting-level")
			if len(vals) == 0 {
				return ""
			}
			return vals[0]
		},
		DisableFlowControl: disableFlowControl,
	})
	grpchantesting.RegisterTestServiceServer(ts, svc)
	// recursive: tunnels can be run on top of tunnels
	// (not realistic, but fun exercise to verify soundness of implementation)
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
	t.Cleanup(func() {
		gs.Stop()
		<-serveDone
	})

	cc, err := grpc.Dial(l.Addr().String(), grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "failed to create client")
	t.Cleanup(func() {
		err := cc.Close()
		require.NoError(t, err, "failed to close client conn")
	})

	// Make sure any goroutines used by the client and server created above have started. That
	// way, we don't incorrectly think they are leaked goroutines.
	time.Sleep(200 * time.Millisecond)

	return tunnelpb.NewTunnelServiceClient(cc), ts
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
