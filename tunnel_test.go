package grpctunnel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/fullstorydev/grpchan/grpchantesting"
	"github.com/fullstorydev/grpchan/inprocgrpc"
	"github.com/jhump/grpctunnel/internal"
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
	cli, ts := setupServer(t, &svr)
	runTests(
		t.Context(), t, modeRunNested, false, cli, ts, &svr,
		func(_ context.Context, t *testing.T, ch grpc.ClientConnInterface) {
			grpchantesting.RunChannelTestCases(t, ch, true)
		},
	)
}

func TestTunnelServiceHandler_Deadlocks(t *testing.T) {
	testCases := []struct {
		name string
		opts []TunnelOption
	}{
		{
			name: "default",
		},
		{
			name: "tiny-chunk",
			opts: []TunnelOption{WithMaxChunkSize(64)},
		},
		{
			name: "oversized-chunk",
			opts: []TunnelOption{WithMaxChunkSize(1024 * 1024 * 1024)},
		},
		{
			name: "tiny-update",
			opts: []TunnelOption{WithMinWindowUpdateSize(1)},
		},
		{
			name: "oversized-update",
			opts: []TunnelOption{WithMinWindowUpdateSize(1024 * 1024 * 1024)},
		},
		{
			name: "all-overridden",
			opts: []TunnelOption{WithInitialWindowSize(16 * 1024), WithMaxChunkSize(4 * 1024), WithMinWindowUpdateSize(4 * 1024)},
		},
	}
	for _, serverCase := range testCases {
		t.Run("server="+serverCase.name, func(t *testing.T) {
			for _, clientCase := range testCases {
				t.Run("client="+clientCase.name, func(t *testing.T) {
					t.Parallel()
					// Each combination runs in its own bubble, with its own server, so
					// the timeouts and delays in these tests use fake time and so that
					// concurrent combinations can't interfere with each other.
					synctest.Test(t, func(t *testing.T) {
						var svr grpchantesting.TestServer
						cli, ts := setupInProcessServer(&svr, serverCase.opts...)
						runTests(
							t.Context(), t, modeRunNested, true, cli, ts, &svr,
							func(ctx context.Context, t *testing.T, ch grpc.ClientConnInterface) {
								runDeadlockTests(ctx, t, ch)
							},
							clientCase.opts...,
						)
						// The test server's handlers ignore cancellation, so some may
						// still be sleeping. Let them finish so synctest.Test doesn't
						// report them as leaked.
						time.Sleep(5 * time.Second)
					})
				})
			}
		})
	}
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
	inBubble bool,
	cl tunnelpb.TunnelServiceClient,
	ts *TunnelServiceHandler,
	testSvr *grpchantesting.TestServer,
	testFunc func(ctx context.Context, t *testing.T, ch grpc.ClientConnInterface),
	opts ...TunnelOption,
) {
	prefix := ""
	if mode == modeIsNested {
		prefix = "nested-"
		ctx = metadata.AppendToOutgoingContext(ctx, "nesting-level", "1")
	}

	runSubtest := func(name string, fn func(t *testing.T)) {
		if inBubble {
			// T.Run may not be called inside a synctest bubble, so run inline.
			// Goroutine leaks are instead detected by synctest.Test, which fails
			// if any goroutines in the bubble are still blocked when it exits.
			// TODO: That only catches leaks at the end of the whole bubble, not
			// per subtest. We could restore per-subtest checks by running fn
			// via pprof.Do with a unique label (goroutines inherit labels from
			// their creator) and polling the goroutine profile until no
			// goroutines with that label remain.
			// TODO: If a leaked goroutine is blocked on a mutex (or anything
			// else synctest doesn't consider "durably blocked"), synctest.Test
			// hangs instead of failing, until "go test -timeout" fires. A
			// real-time watchdog started outside the bubble could fail fast
			// with a goroutine dump instead.
			t.Logf("running %s", name)
			fn(t)
			return
		}
		t.Run(name, func(t *testing.T) {
			checkForGoroutineLeak(t, func() { fn(t) })
		})
	}

	runSubtest(prefix+"forward", func(t *testing.T) {
		ch, err := NewChannel(cl, opts...).Start(ctx)
		require.NoError(t, err, "failed to open tunnel")

		defer func() {
			ch.Close()
			<-ch.Done()
			assert.NoError(t, ch.Err(), "channel ended with error")
		}()

		testFunc(ctx, t, ch)

		if mode == modeRunNested {
			// nested/recursive test
			runTests(
				ch.Context(), t, modeIsNested, inBubble,
				tunnelpb.NewTunnelServiceClient(ch),
				ts, testSvr, testFunc,
				opts...,
			)
		}
	})

	runSubtest(prefix+"reverse", func(t *testing.T) {
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
				ctx, t, modeIsNested, inBubble,
				tunnelpb.NewTunnelServiceClient(ch),
				ts, testSvr, testFunc,
				opts...,
			)
		}

		for i, rt := range ts.AllReverseTunnels() {
			assert.NoError(t, rt.Err(), "reverse tunnel channel %d ended with error", i)
		}
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
		for range 2 {
			err := stream.Send(&grpchantesting.Message{
				DelayMillis: 1000,
				Payload:     bytes.Repeat([]byte{0, 1, 2, 3}, 10_000),
			})
			if err != nil {
				require.Error(t, ctx.Err())
				break
			}
		}
	}()
	time.Sleep(100 * time.Millisecond) // make sure the slow one has had time to issue its RPC

	grp, ctx := errgroup.WithContext(ctx)
	for range 10 {
		grp.Go(func() error {
			// this should proceed just fine, regardless of the slow one
			stream, err := stub.ClientStream(ctx)
			if err != nil {
				return err
			}
			for range 20 {
				err := stream.Send(&grpchantesting.Message{
					Payload: bytes.Repeat([]byte{0, 1, 2, 3}, 5_000),
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
	var svr grpchantesting.TestServer
	tunnelCli, ts := setupServer(t, &svr)

	forwardCh, err := NewChannel(tunnelCli).Start(t.Context())
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
		started, err := revSvr.Serve(t.Context())
		assert.True(t, started, "ReverseTunnelServer.Serve returned false")
		assert.NoError(t, err, "ReverseTunnelServer.Serve returned error")
	}()
	defer func() {
		revSvr.Stop()
		<-serveDone
	}()

	// make sure server has registered client, so we can issue RPCs to it
	reverseCh := ts.AsChannel()
	timedCtx, cancel := context.WithTimeout(t.Context(), time.Second)
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
			var count atomic.Int32
			runOneThread := func() {
				for {
					select {
					case <-done:
						return
					default:
					}
					_, err := cli.Unary(t.Context(), &grpchantesting.Message{})
					if !assert.NoError(t, err) {
						return
					}
					count.Add(1)
				}
			}

			// Ten goroutines all using the same tunnel, hoping to catch data races or
			// other concurrency-related bugs.
			checkForGoroutineLeak(t, func() {
				var wg sync.WaitGroup
				for range 10 {
					wg.Go(runOneThread)
				}
				// all threads sending concurrent requests for 3 seconds
				time.Sleep(2 * time.Second)
				close(done)
				wg.Wait()
			})

			t.Logf("RPCs sent: %d", count.Load())
		})
	}
}

func TestTunnelServiceHandler_TrailersAvailableAtEOF(t *testing.T) {
	// Regression test: trailers must be available as soon as the client
	// observes the end of the stream. Previously, the end of the stream could
	// be observed slightly before trailers were recorded.
	var svr grpchantesting.TestServer
	tunnelCli, _ := setupInProcessServer(&svr)
	ch, err := NewChannel(tunnelCli).Start(t.Context())
	require.NoError(t, err)
	defer func() {
		ch.Close()
		<-ch.Done()
	}()
	stub := grpchantesting.NewTestServiceClient(ch)
	// The race window is narrow, so we use lots of concurrent RPCs to make
	// it more likely to be hit.
	grp, ctx := errgroup.WithContext(t.Context())
	for range 20 {
		grp.Go(func() error {
			for range 500 {
				stream, err := stub.ServerStream(ctx, &grpchantesting.Message{
					Trailers: map[string][]byte{"foo": []byte("bar")},
				})
				if err != nil {
					return err
				}
				if _, err := stream.Recv(); !errors.Is(err, io.EOF) {
					return fmt.Errorf("expected EOF, got %w", err)
				}
				if trailer := stream.Trailer().Get("foo"); len(trailer) != 1 || trailer[0] != "bar" {
					return fmt.Errorf("wrong trailer at EOF: %v", trailer)
				}
			}
			return nil
		})
	}
	require.NoError(t, grp.Wait())
}

// TODO: also need more tests around channel lifecycle, and ensuring it
// properly respects things like context cancellations, etc

func newTestHandler(svc grpchantesting.TestServiceServer, opts ...TunnelOption) *TunnelServiceHandler {
	var options tunnelOpts
	for _, opt := range opts {
		opt.apply(&options)
	}
	ts := NewTunnelServiceHandler(TunnelServiceHandlerOptions{
		AffinityKey: func(t TunnelChannel) any {
			md, _ := metadata.FromIncomingContext(t.Context())
			vals := md.Get("nesting-level")
			if len(vals) == 0 {
				return ""
			}
			return vals[0]
		},
		InitialWindowSize:   options.initialWindowSize,
		MaxChunkSize:        options.maxChunkSize,
		MinWindowUpdateSize: options.minWindowUpdateSize,
	})
	grpchantesting.RegisterTestServiceServer(ts, svc)
	// recursive: tunnels can be run on top of tunnels
	// (not realistic, but fun exercise to verify soundness of implementation)
	tunnelpb.RegisterTunnelServiceServer(ts, ts.Service())
	return ts
}

// setupInProcessServer is like setupServer, except that the tunnel service is
// exposed via an in-process channel instead of over the network. Unlike
// setupServer, this can be used inside a synctest bubble.
func setupInProcessServer(svc grpchantesting.TestServiceServer, opts ...TunnelOption) (tunnelpb.TunnelServiceClient, *TunnelServiceHandler) {
	ts := newTestHandler(svc, opts...)
	var ch inprocgrpc.Channel
	tunnelpb.RegisterTunnelServiceServer(&ch, ts.Service())
	return tunnelpb.NewTunnelServiceClient(&ch), ts
}

func setupServer(t *testing.T, svc grpchantesting.TestServiceServer, opts ...TunnelOption) (tunnelpb.TunnelServiceClient, *TunnelServiceHandler) {
	ts := newTestHandler(svc, opts...)

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

	cc, err := internal.BlockingDial(t.Context(), l.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
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
