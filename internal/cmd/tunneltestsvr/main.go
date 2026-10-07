package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/fullstorydev/grpchan/grpchantesting"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/jhump/grpctunnel"
	"github.com/jhump/grpctunnel/internal"
	"github.com/jhump/grpctunnel/internal/gen"
	"github.com/jhump/grpctunnel/tunnelpb"
)

func main() {
	port := flag.Int("port", 26354, "the port on which this server will listen")
	flag.Parse()

	svr := grpc.NewServer()
	tunnelSvc := grpctunnel.NewTunnelServiceHandler(grpctunnel.TunnelServiceHandlerOptions{
		AffinityKey: func(ch grpctunnel.TunnelChannel) any {
			md, _ := metadata.FromIncomingContext(ch.Context())
			vals := md.Get("test-client-key")
			if len(vals) == 0 {
				return ""
			}
			return vals[0]
		},
	})
	tunnelpb.RegisterTunnelServiceServer(svr, tunnelSvc.Service())
	gen.RegisterTunnelTestServiceServer(svr, &tunnelTester{tunnelSvc: tunnelSvc})

	// Over the tunnel, we just expose this simple test service
	grpchantesting.RegisterTestServiceServer(tunnelSvc, &grpchantesting.TestServer{})

	lis, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Listening on", lis.Addr().String())
	// This only returns (and thus program exits) on failure.
	// Otherwise, process is stopped via signal.
	if err := svr.Serve(lis); err != nil {
		log.Fatal(err)
	}
}

type tunnelTester struct {
	gen.UnimplementedTunnelTestServiceServer
	tunnelSvc *grpctunnel.TunnelServiceHandler
}

func (tt *tunnelTester) TriggerTestRPCs(ctx context.Context, req *gen.TriggerTestRPCsRequest) (*gen.TriggerTestRPCsResponse, error) {
	ch := tt.tunnelSvc.KeyAsChannel(req.ClientKey)
	waitCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := ch.WaitForReady(waitCtx); err != nil {
		return nil, err
	}
	log.Println("Sending RPCs over reverse tunnel to", req.ClientKey)
	if err := internal.SendRPCs(ctx, grpchantesting.NewTestServiceClient(ch)); err != nil {
		return nil, err
	}
	return &gen.TriggerTestRPCsResponse{}, nil
}
