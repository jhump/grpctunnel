// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             (unknown)
// source: grpctunnel/testing/testing.proto

package gen

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	TunnelTestService_TriggerTestRPCs_FullMethodName = "/TunnelTestService/TriggerTestRPCs"
)

// TunnelTestServiceClient is the client API for TunnelTestService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type TunnelTestServiceClient interface {
	// This is invoked on a server after a client establishes a reverse tunnel.
	// This will use the reverse tunnel to send a bunch of RPCs, from the server
	// to the client, and then return when finished.
	TriggerTestRPCs(ctx context.Context, in *TriggerTestRPCsRequest, opts ...grpc.CallOption) (*TriggerTestRPCsResponse, error)
}

type tunnelTestServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewTunnelTestServiceClient(cc grpc.ClientConnInterface) TunnelTestServiceClient {
	return &tunnelTestServiceClient{cc}
}

func (c *tunnelTestServiceClient) TriggerTestRPCs(ctx context.Context, in *TriggerTestRPCsRequest, opts ...grpc.CallOption) (*TriggerTestRPCsResponse, error) {
	out := new(TriggerTestRPCsResponse)
	err := c.cc.Invoke(ctx, TunnelTestService_TriggerTestRPCs_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// TunnelTestServiceServer is the server API for TunnelTestService service.
// All implementations must embed UnimplementedTunnelTestServiceServer
// for forward compatibility
type TunnelTestServiceServer interface {
	// This is invoked on a server after a client establishes a reverse tunnel.
	// This will use the reverse tunnel to send a bunch of RPCs, from the server
	// to the client, and then return when finished.
	TriggerTestRPCs(context.Context, *TriggerTestRPCsRequest) (*TriggerTestRPCsResponse, error)
	mustEmbedUnimplementedTunnelTestServiceServer()
}

// UnimplementedTunnelTestServiceServer must be embedded to have forward compatible implementations.
type UnimplementedTunnelTestServiceServer struct {
}

func (UnimplementedTunnelTestServiceServer) TriggerTestRPCs(context.Context, *TriggerTestRPCsRequest) (*TriggerTestRPCsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method TriggerTestRPCs not implemented")
}
func (UnimplementedTunnelTestServiceServer) mustEmbedUnimplementedTunnelTestServiceServer() {}

// UnsafeTunnelTestServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to TunnelTestServiceServer will
// result in compilation errors.
type UnsafeTunnelTestServiceServer interface {
	mustEmbedUnimplementedTunnelTestServiceServer()
}

func RegisterTunnelTestServiceServer(s grpc.ServiceRegistrar, srv TunnelTestServiceServer) {
	s.RegisterService(&TunnelTestService_ServiceDesc, srv)
}

func _TunnelTestService_TriggerTestRPCs_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TriggerTestRPCsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TunnelTestServiceServer).TriggerTestRPCs(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TunnelTestService_TriggerTestRPCs_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TunnelTestServiceServer).TriggerTestRPCs(ctx, req.(*TriggerTestRPCsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// TunnelTestService_ServiceDesc is the grpc.ServiceDesc for TunnelTestService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var TunnelTestService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "TunnelTestService",
	HandlerType: (*TunnelTestServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "TriggerTestRPCs",
			Handler:    _TunnelTestService_TriggerTestRPCs_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "grpctunnel/testing/testing.proto",
}
