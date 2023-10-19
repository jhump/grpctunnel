package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/fullstorydev/grpchan"
	"log"
	"sync/atomic"
	"time"

	"github.com/fullstorydev/grpchan/grpchantesting"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/jhump/grpctunnel"
	"github.com/jhump/grpctunnel/internal"
	"github.com/jhump/grpctunnel/internal/gen"
	"github.com/jhump/grpctunnel/tunnelpb"
)

func main() {
	serverPort := flag.Int("server-port", 26354, "the port on which the server is listening")
	flag.Parse()

	ctx := context.Background()
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cc, err := grpc.DialContext(dialCtx,
		fmt.Sprintf("127.0.0.1:%d", *serverPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithReturnConnectionError(),
	)
	if err != nil {
		log.Fatal(err)
	}

	// First check the forward tunnel.
	tunnelClient := tunnelpb.NewTunnelServiceClient(cc)
	tc, err := tunnelClient.OpenTunnel(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Tunnel created.")
	tunnel := grpctunnel.NewChannel(tc)
	defer tunnel.Close()
	var clCounts atomic.Int32
	if err := internal.SendRPCs(ctx, grpchantesting.NewTestServiceClient(withClientCounts(tunnel, &clCounts))); err != nil {
		log.Fatal(err)
	}
	log.Printf("Issued %d requests over tunnel.", clCounts.Load())

	// Then check the reverse tunnel.
	key, err := makeClientKey()
	if err != nil {
		log.Fatal(err)
	}
	reverseTunnel := grpctunnel.NewReverseTunnelServer(tunnelClient)
	// Over the tunnel, we just expose this simple test service
	var svrCounts atomic.Int32
	grpchantesting.RegisterTestServiceServer(withServerCounts(reverseTunnel, &svrCounts), &grpchantesting.TestServer{})
	ctx, cancel = context.WithCancel(ctx)
	done := make(chan struct{})
	defer func() {
		cancel()
		<-done
	}()
	go func() {
		defer close(done)
		ctx := metadata.AppendToOutgoingContext(ctx, "test-client-key", key)
		if started, err := reverseTunnel.Serve(ctx); !started {
			log.Fatal(err)
		}
	}()
	log.Printf("Reverse tunnel started (key = %s).", key)
	// This tells the server to initiate RPCs via
	tester := gen.NewTunnelTestServiceClient(cc)
	if _, err := tester.TriggerTestRPCs(ctx, &gen.TriggerTestRPCsRequest{ClientKey: key}); err != nil {
		log.Fatal(err)
	}
	log.Printf("Served %d requests over reverse tunnel.", svrCounts.Load())

	// Success!
}

func makeClientKey() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func withClientCounts(ch grpc.ClientConnInterface, counts *atomic.Int32) grpc.ClientConnInterface {
	return grpchan.InterceptClientConn(
		ch,
		func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			counts.Add(1)
			return invoker(ctx, method, req, reply, cc, opts...)
		},
		func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			counts.Add(1)
			return streamer(ctx, desc, cc, method, opts...)
		},
	)
}

func withServerCounts(reg grpc.ServiceRegistrar, counts *atomic.Int32) grpc.ServiceRegistrar {
	return grpchan.WithInterceptor(
		reg,
		func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
			counts.Add(1)
			return handler(ctx, req)
		},
		func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			counts.Add(1)
			return handler(srv, ss)
		},
	)
}
