package grpctunnel

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type (
	tunnelMetadataIncomingContextKey struct{}
	tunnelMetadataOutgoingContextKey struct{}
	tunnelChannelContextKey          struct{}
)

// TunnelMetadataFromIncomingContext provides server-side access to the request
// metadata used to open a tunnel. This can be used from server interceptors or
// handlers that are handling tunneled requests in a tunnel.
func TunnelMetadataFromIncomingContext(ctx context.Context) (metadata.MD, bool) {
	md, ok := ctx.Value(tunnelMetadataIncomingContextKey{}).(metadata.MD)
	return md.Copy(), ok
}

// TunnelMetadataFromOutgoingContext provides client-side access to the request
// metadata used to open a forward tunnel. This can be used from client
// interceptors that are processing tunneled requests in a forward tunnel.
//
// For client interceptors in a reverse tunnel, you can use
// [metadata.FromOutgoingContext] with the context passed to the interceptor or
// handler.
func TunnelMetadataFromOutgoingContext(ctx context.Context) (metadata.MD, bool) {
	md, ok := ctx.Value(tunnelMetadataOutgoingContextKey{}).(metadata.MD)
	return md.Copy(), ok
}

// TunnelChannelFromContext returns the TunnelChannel that is handling the given
// request context. If the given context is not a client-side request context,
// or if the channel for the request is not a tunnel, this will return nil.
func TunnelChannelFromContext(ctx context.Context) TunnelChannel {
	tc, _ := ctx.Value(tunnelChannelContextKey{}).(TunnelChannel)
	return tc
}

// WithTunnelChannel provides the caller access to the specific TunnelChanel
// that was used to send a tunneled RPC. This is similar to using
// TunnelChannelFromContext except it can be used with unary RPCs, where the
// invoker never has access to values added to the request context. When the RPC
// completes, the given location will be updated with the channel that handled
// the request. If the channel that handled the request was not a tunnel, the
// location is left unchanged.
//
// The pointer must point to an allocated location. Passing a nil pointer will
// result in a panic when an RPC is invoked with the returned option.
func WithTunnelChannel(ch *TunnelChannel) grpc.CallOption {
	return &tunnelChannelCallOption{ch: ch}
}

type tunnelChannelCallOption struct {
	ch *TunnelChannel
	grpc.EmptyCallOption
}
