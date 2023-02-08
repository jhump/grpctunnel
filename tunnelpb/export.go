// Package tunnelpb contains generated code corresponding to the Protocol
// Buffer definition of the tunneling protocol.
package tunnelpb

//go:generate bash -c "cd ../proto && buf generate"

// ClientToServerFrame is the type that is assignable to ClientToServer.Frame.
type ClientToServerFrame = isClientToServer_Frame

// ServerToClientFrame is the type that is assignable to ServerToClient.Frame.
type ServerToClientFrame = isServerToClient_Frame
