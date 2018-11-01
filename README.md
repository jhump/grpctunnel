# gRPC Tunnels

## Still Under Construction!

This library enables carrying gRPC over gRPC. There are a few niche use cases where this could be useful, but the most widely applicable one is likely for letting gRPC servers communicate in the reverse direction, sending requests to connected clients.

The tunnel is itself a gRPC service, which provides bidirectional streaming methods for forward and reverse tunneling. There is also API for easily configuring the server handlers, be it on the server or (in the case of reverse tunnels) on the client. Similarly, there is API for getting a "channel", from which you can create service stubs. This allows the code that uses the stubs to not even care whether it has a normal gRPC client connection or a stub that sends the data via a tunnel.

There is also API for "light-weight" tunneling, which is where a custom bidirectional stream can be used to send messages back and forth, where the messages each act as RPC requests and responses, but on a single stream (for pinning/affinity, for example).
