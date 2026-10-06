package grpctunnel

import (
	"testing"
	"time"

	"github.com/fullstorydev/grpchan/grpchantesting"
	"github.com/fullstorydev/grpchan/inprocgrpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/jhump/grpctunnel/tunnelpb"
)

func TestTunnelServer_RejectsBadNewStream(t *testing.T) {
	validNewStream := func() *tunnelpb.NewStream {
		return &tunnelpb.NewStream{
			MethodName:        "grpchantesting.TestService/Unary",
			ProtocolRevision:  tunnelpb.ProtocolRevision_REVISION_ONE,
			InitialWindowSize: defaultInitialWindowSize,
		}
	}
	testCases := []struct {
		name string
		// mutates a valid NewStream frame to make it invalid
		modify func(*tunnelpb.NewStream)
		// if true, the server is shutting down
		shuttingDown bool
		expectCode   codes.Code
		expectMsg    string
	}{
		{
			name:       "zero-initial-window-size",
			modify:     func(ns *tunnelpb.NewStream) { ns.InitialWindowSize = 0 },
			expectCode: codes.Internal,
			expectMsg:  "initial window size",
		},
		{
			name:       "revision-zero",
			modify:     func(ns *tunnelpb.NewStream) { ns.ProtocolRevision = tunnelpb.ProtocolRevision_REVISION_ZERO },
			expectCode: codes.Unavailable,
			expectMsg:  "upgrade client",
		},
		{
			name:       "unknown-revision",
			modify:     func(ns *tunnelpb.NewStream) { ns.ProtocolRevision = 99 },
			expectCode: codes.Unavailable,
			expectMsg:  "does not support protocol revision 99",
		},
		{
			name:       "empty-method-name",
			modify:     func(ns *tunnelpb.NewStream) { ns.MethodName = "" },
			expectCode: codes.InvalidArgument,
			expectMsg:  "not a well-formed method name",
		},
		{
			name:       "malformed-method-name",
			modify:     func(ns *tunnelpb.NewStream) { ns.MethodName = "foo" },
			expectCode: codes.InvalidArgument,
			expectMsg:  "not a well-formed method name",
		},
		{
			name:         "server-shutting-down",
			modify:       func(*tunnelpb.NewStream) {},
			shuttingDown: true,
			expectCode:   codes.Unavailable,
			expectMsg:    "shutting down",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var svr grpchantesting.TestServer
			tunnelCli, ts := setupInProcessServer(&svr)
			ctx := metadata.AppendToOutgoingContext(t.Context(), grpctunnelNegotiateKey, grpctunnelNegotiateVal)
			stream, err := tunnelCli.OpenTunnel(ctx)
			require.NoError(t, err)
			if testCase.shuttingDown {
				ts.InitiateShutdown()
			}

			// Like a real client, we send more frames for the stream right after
			// the NewStream frame, without waiting to see if it was accepted.
			newStream := validNewStream()
			testCase.modify(newStream)
			sendFrames(t, stream,
				&tunnelpb.ClientToServer{
					StreamId: 1,
					Frame:    &tunnelpb.ClientToServer_NewStream{NewStream: newStream},
				},
				&tunnelpb.ClientToServer{
					StreamId: 1,
					Frame:    &tunnelpb.ClientToServer_HalfClose{HalfClose: &emptypb.Empty{}},
				},
			)
			st := awaitCloseStream(t, stream, 1)
			require.Equal(t, testCase.expectCode, st.Code())
			require.Contains(t, st.Message(), testCase.expectMsg)

			// Only that stream should have been rejected. The tunnel should still
			// be usable for other streams, so we should get back a response for
			// another stream (instead of the tunnel being torn down).
			newStream = validNewStream()
			newStream.MethodName = "grpchantesting.TestService/DoesNotExist"
			sendFrames(t, stream,
				&tunnelpb.ClientToServer{
					StreamId: 2,
					Frame:    &tunnelpb.ClientToServer_NewStream{NewStream: newStream},
				},
			)
			st = awaitCloseStream(t, stream, 2)
			if testCase.shuttingDown {
				require.Equal(t, codes.Unavailable, st.Code())
			} else {
				require.Equal(t, codes.Unimplemented, st.Code())
			}
		})
	}
}

func TestTunnelChannel_RejectsZeroInitialWindowSize(t *testing.T) {
	var inproc inprocgrpc.Channel
	tunnelpb.RegisterTunnelServiceServer(&inproc, zeroWindowTunnelServer{})
	ch, err := NewChannel(tunnelpb.NewTunnelServiceClient(&inproc)).Start(t.Context())
	require.NoError(t, err)
	defer ch.Close()

	select {
	case <-ch.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("tunnel was not closed after receiving bad settings")
	}
	require.ErrorContains(t, ch.Err(), "initial window size")
}

// zeroWindowTunnelServer is a tunnel server that advertises an initial
// window size of zero, which is invalid.
type zeroWindowTunnelServer struct {
	tunnelpb.UnimplementedTunnelServiceServer
}

func (zeroWindowTunnelServer) OpenTunnel(stream tunnelpb.TunnelService_OpenTunnelServer) error {
	if err := stream.SendHeader(metadata.Pairs(grpctunnelNegotiateKey, grpctunnelNegotiateVal)); err != nil {
		return err
	}
	err := stream.Send(&tunnelpb.ServerToClient{
		StreamId: -1,
		Frame: &tunnelpb.ServerToClient_Settings{
			Settings: &tunnelpb.Settings{
				SupportedProtocolRevisions: []tunnelpb.ProtocolRevision{tunnelpb.ProtocolRevision_REVISION_ONE},
			},
		},
	})
	if err != nil {
		return err
	}
	// Wait for the client to hang up.
	for {
		if _, err := stream.Recv(); err != nil {
			return nil
		}
	}
}

func sendFrames(t *testing.T, stream tunnelpb.TunnelService_OpenTunnelClient, frames ...*tunnelpb.ClientToServer) {
	t.Helper()
	for _, frame := range frames {
		require.NoError(t, stream.Send(frame))
	}
}

// awaitCloseStream reads frames from the given tunnel until it receives a
// CloseStream frame for the given stream ID and then returns its status. Other
// frames, like the server's settings, are ignored.
func awaitCloseStream(t *testing.T, stream tunnelpb.TunnelService_OpenTunnelClient, streamID int64) *status.Status {
	t.Helper()
	for {
		in, err := stream.Recv()
		require.NoError(t, err)
		if closeStream, ok := in.Frame.(*tunnelpb.ServerToClient_CloseStream); ok && in.StreamId == streamID {
			return status.FromProto(closeStream.CloseStream.Status)
		}
	}
}
