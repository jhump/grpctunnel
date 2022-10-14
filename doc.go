// Package grpctunnel provides APIs to support tunneling of gRPC services:
// carrying gRPC calls over a gRPC stream.
//
// A tunnel is a gRPC stream on which you can send other RPC requests, in
// either direction.
//
// # Forward Tunnels
//
// A "forward" tunnel is a tunnel in the normal direction, where the gRPC client
// initiates RPCs by sending requests, and the gRPC server acts on them and
// responses.
//
// Forward tunnels allow a client to pin RPCs to a single server since they are
// all sent over a single stream. Forward tunnels work like so:
//
//   - Client issues an RPC that establishes the forward tunnel. The RPC is a
//     full-duplex bidirectional stream, so can support all manner of streaming
//     RPCs over the tunnel.
//   - Client then uses the tunnel to create a new gRPC client connection. (See
//     NewChannel).
//   - RPC stubs can then be created using this new connection. All RPCs issued
//     on this connection are transmitted over the tunnel, on the stream that was
//     established in step 1.
//   - Closing the tunnel channel also results in the underlying stream
//     being closed.
//
// # Reverse Tunnels
//
// A "reverse" tunnel is a tunnel in the opposite direction of normal: the gRPC
// server is the actor that initiates RPCs by sending requests, and a gRPC
// client handles these requests and sends responses.
//
// Reverse tunnels allow for interesting "server push" scenarios, where the
// pushed messages can be more advanced than "fire and forget" but actually need
// responses/acknowledgements. Reverse tunnels work like so:
//
//   - Client issues an RPC that establishes the reverse tunnel. The RPC is a
//     full-duplex bidirectional stream, so can support all manner of streaming
//     RPCs over the tunnel. (See NewReverseTunnelServer.)
//   - Server registers the reverse tunnel for use as a channel. (See relevant
//     fields in TunnelServiceHandlerOptions.)
//   - RPC stubs can be created inside the server, using these reverse tunnel
//     channels. All RPCs issued on this channel are transmitted over the
//     tunnel, on the stream that was established in step 1.
//   - Shutting down the reverse tunnel server (which is in the client process
//     that established the tunnels) will close all tunnels and their underlying
//     streams.
//
// # Service Handler
//
// The TunnelServiceHandler is what implements the tunneling protocol. You can
// register the RPC services available for forward tunnels with it. You can also
// use it to access reverse tunnels, for the server to send RPCs back to the
// client. See NewTunnelServiceHandler.
//
// This is the value that is registered with a *grpc.Server to expose the
// Tunnel service.
package grpctunnel
