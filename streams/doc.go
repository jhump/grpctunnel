// Package streams contains code for adapting common patterns for full-duplex
// bidirectional streams to more user-friendly APIs.
//
// Often these kinds of APIs have a way for one side to send a message and the
// other to acknowledge it. Since this resembles the typical request-response
// flow of a non-streaming RPC, an API that more closely models this
// request-response flow will be much more familiar to users and easier to use.
//
// Like tunneling, this stream adaptation works in either direction: the typical
// "forward" direction, where the gRPC client (that initiated the stream) sends
// requests and receives responses; or the "reverse" direction, where the gRPC
// client receives requests from the server and sends responses.
//
// The effective client (whichever side is initiating operations on the
// stream, be it a gRPC client or a gRPC server) uses NewStreamAdapter and its
// Call method to send requests. The effective server uses HandleServerStream
// to dispatch requests and send responses.
//
// These provide fairly low-level APIs, so it is expected that a nicer API be
// built on top. The APIs in this package mostly provide the mechanics for
// request-response correlation and the ability for a request sender to await
// a response. See the 'examples' subdirectory for example usages that
// demonstrate how this package is meant to be used.
package streams
