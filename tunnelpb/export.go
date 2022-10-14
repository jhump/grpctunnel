// Package tunnelpb contains generated code corresponding to the Protocol
// Buffer definition of the tunneling protocol.
package tunnelpb

//go:generate bash -c "cd ../proto && buf generate"

// These are exported for convenience to implementation code in grpctunnel package.
type IsClientToServer_Frame = isClientToServer_Frame
type IsServerToClient_Frame = isServerToClient_Frame
