# Stream Adapter Examples

This directory contains two examples, showing how to use the API in the
"github.com/jhump/grpctunnel/streams" package.

1. The first example, `reflection`, uses the stream adapters to provide a
   simpler API for the gRPC server reflection API. (Note that this is just an
   example. For production services, it is instead recommended to use the
   `Client` type in "github.com/jhump/protoreflect/grpcreflect".)
2. The second example, `calculator`, is a simple test service that provides
   a calculator interface, where operations can be applied to in-memory
   variables via a streaming interface.
