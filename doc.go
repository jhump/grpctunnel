// Package grpctunnel provides tools to support tunneling of gRPC services:
// carrying gRPC calls over a gRPC stream.
//
// This support includes "pinning" an RPC channel to a single server, by sending
// all requests on a single gRPC stream. There are also tools for adapting
// certain kinds of bidirectional stream RPCs into a stub such that a single
// stream looks like a sequence of unary calls.
//
// This support also includes "reverse services", where a client can initiate a
// connection to a server and subsequently the server can then wrap that
// connection with an RPC stub, used to send requests from the server to that
// client (and the client then replies and sends responses back to the server).
package grpctunnel

//go:generate bash -c "cd proto && buf generate"
