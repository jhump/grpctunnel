// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             (unknown)
// source: grpctunnel/v1/tunnel.proto

package tunnelpb

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
	TunnelService_OpenTunnel_FullMethodName        = "/grpctunnel.v1.TunnelService/OpenTunnel"
	TunnelService_OpenReverseTunnel_FullMethodName = "/grpctunnel.v1.TunnelService/OpenReverseTunnel"
)

// TunnelServiceClient is the client API for TunnelService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type TunnelServiceClient interface {
	// OpenTunnel creates a channel to the server which can be used to send
	// additional RPCs, all of which will be sent to the same server via a
	// single underlying gRPC stream. This can provide affinity for a "chatty"
	// sequence of calls, where the gRPC connection is load balanced (so there
	// may be multiple backend servers), but a particular "conversation" (which
	// may consist of numerous RPCs) needs to all go to a single server, for
	// consistency.
	OpenTunnel(ctx context.Context, opts ...grpc.CallOption) (TunnelService_OpenTunnelClient, error)
	// OpenReverseTunnel creates a "reverse" channel, which allows the server to
	// act as a client and send RPCs to the client that creates the tunnel. It
	// is in most respects identical to OpenTunnel except that the roles are
	// reversed: the server initiates RPCs and sends requests and the client
	// replies to them and sends responses.
	OpenReverseTunnel(ctx context.Context, opts ...grpc.CallOption) (TunnelService_OpenReverseTunnelClient, error)
}

type tunnelServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewTunnelServiceClient(cc grpc.ClientConnInterface) TunnelServiceClient {
	return &tunnelServiceClient{cc}
}

func (c *tunnelServiceClient) OpenTunnel(ctx context.Context, opts ...grpc.CallOption) (TunnelService_OpenTunnelClient, error) {
	stream, err := c.cc.NewStream(ctx, &TunnelService_ServiceDesc.Streams[0], TunnelService_OpenTunnel_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &tunnelServiceOpenTunnelClient{stream}
	return x, nil
}

type TunnelService_OpenTunnelClient interface {
	Send(*ClientToServer) error
	Recv() (*ServerToClient, error)
	grpc.ClientStream
}

type tunnelServiceOpenTunnelClient struct {
	grpc.ClientStream
}

func (x *tunnelServiceOpenTunnelClient) Send(m *ClientToServer) error {
	return x.ClientStream.SendMsg(m)
}

func (x *tunnelServiceOpenTunnelClient) Recv() (*ServerToClient, error) {
	m := new(ServerToClient)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *tunnelServiceClient) OpenReverseTunnel(ctx context.Context, opts ...grpc.CallOption) (TunnelService_OpenReverseTunnelClient, error) {
	stream, err := c.cc.NewStream(ctx, &TunnelService_ServiceDesc.Streams[1], TunnelService_OpenReverseTunnel_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &tunnelServiceOpenReverseTunnelClient{stream}
	return x, nil
}

type TunnelService_OpenReverseTunnelClient interface {
	Send(*ServerToClient) error
	Recv() (*ClientToServer, error)
	grpc.ClientStream
}

type tunnelServiceOpenReverseTunnelClient struct {
	grpc.ClientStream
}

func (x *tunnelServiceOpenReverseTunnelClient) Send(m *ServerToClient) error {
	return x.ClientStream.SendMsg(m)
}

func (x *tunnelServiceOpenReverseTunnelClient) Recv() (*ClientToServer, error) {
	m := new(ClientToServer)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// TunnelServiceServer is the server API for TunnelService service.
// All implementations must embed UnimplementedTunnelServiceServer
// for forward compatibility
type TunnelServiceServer interface {
	// OpenTunnel creates a channel to the server which can be used to send
	// additional RPCs, all of which will be sent to the same server via a
	// single underlying gRPC stream. This can provide affinity for a "chatty"
	// sequence of calls, where the gRPC connection is load balanced (so there
	// may be multiple backend servers), but a particular "conversation" (which
	// may consist of numerous RPCs) needs to all go to a single server, for
	// consistency.
	OpenTunnel(TunnelService_OpenTunnelServer) error
	// OpenReverseTunnel creates a "reverse" channel, which allows the server to
	// act as a client and send RPCs to the client that creates the tunnel. It
	// is in most respects identical to OpenTunnel except that the roles are
	// reversed: the server initiates RPCs and sends requests and the client
	// replies to them and sends responses.
	OpenReverseTunnel(TunnelService_OpenReverseTunnelServer) error
	mustEmbedUnimplementedTunnelServiceServer()
}

// UnimplementedTunnelServiceServer must be embedded to have forward compatible implementations.
type UnimplementedTunnelServiceServer struct {
}

func (UnimplementedTunnelServiceServer) OpenTunnel(TunnelService_OpenTunnelServer) error {
	return status.Errorf(codes.Unimplemented, "method OpenTunnel not implemented")
}
func (UnimplementedTunnelServiceServer) OpenReverseTunnel(TunnelService_OpenReverseTunnelServer) error {
	return status.Errorf(codes.Unimplemented, "method OpenReverseTunnel not implemented")
}
func (UnimplementedTunnelServiceServer) mustEmbedUnimplementedTunnelServiceServer() {}

// UnsafeTunnelServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to TunnelServiceServer will
// result in compilation errors.
type UnsafeTunnelServiceServer interface {
	mustEmbedUnimplementedTunnelServiceServer()
}

func RegisterTunnelServiceServer(s grpc.ServiceRegistrar, srv TunnelServiceServer) {
	s.RegisterService(&TunnelService_ServiceDesc, srv)
}

func _TunnelService_OpenTunnel_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(TunnelServiceServer).OpenTunnel(&tunnelServiceOpenTunnelServer{stream})
}

type TunnelService_OpenTunnelServer interface {
	Send(*ServerToClient) error
	Recv() (*ClientToServer, error)
	grpc.ServerStream
}

type tunnelServiceOpenTunnelServer struct {
	grpc.ServerStream
}

func (x *tunnelServiceOpenTunnelServer) Send(m *ServerToClient) error {
	return x.ServerStream.SendMsg(m)
}

func (x *tunnelServiceOpenTunnelServer) Recv() (*ClientToServer, error) {
	m := new(ClientToServer)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _TunnelService_OpenReverseTunnel_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(TunnelServiceServer).OpenReverseTunnel(&tunnelServiceOpenReverseTunnelServer{stream})
}

type TunnelService_OpenReverseTunnelServer interface {
	Send(*ClientToServer) error
	Recv() (*ServerToClient, error)
	grpc.ServerStream
}

type tunnelServiceOpenReverseTunnelServer struct {
	grpc.ServerStream
}

func (x *tunnelServiceOpenReverseTunnelServer) Send(m *ClientToServer) error {
	return x.ServerStream.SendMsg(m)
}

func (x *tunnelServiceOpenReverseTunnelServer) Recv() (*ServerToClient, error) {
	m := new(ServerToClient)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// TunnelService_ServiceDesc is the grpc.ServiceDesc for TunnelService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var TunnelService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "grpctunnel.v1.TunnelService",
	HandlerType: (*TunnelServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "OpenTunnel",
			Handler:       _TunnelService_OpenTunnel_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		{
			StreamName:    "OpenReverseTunnel",
			Handler:       _TunnelService_OpenReverseTunnel_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "grpctunnel/v1/tunnel.proto",
}
