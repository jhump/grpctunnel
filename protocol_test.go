package grpctunnel

import (
	"context"
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

// TODO: The tests in this file verify the protocol as implemented by this
// version of the package. They don't verify interoperability with older
// versions. That is currently done manually, using the tunneltestsvr and
// tunneltestclient programs (in internal/cmd), built from both the current
// code and from older releases, and running every combination of client and
// server. We could automate that by building the older programs in a test,
// e.g. "go install github.com/jhump/grpctunnel/internal/cmd/tunneltestsvr@v0.3.0".

func TestTunnelServer_AcceptsSupportedRevisions(t *testing.T) {
	for _, revision := range supportedRevisions {
		t.Run(revision.String(), func(t *testing.T) {
			var svr grpchantesting.TestServer
			tunnelCli, _ := setupInProcessServer(&svr)
			ctx := metadata.AppendToOutgoingContext(t.Context(), grpctunnelNegotiateKey, grpctunnelNegotiateVal)
			stream, err := tunnelCli.OpenTunnel(ctx)
			require.NoError(t, err)

			// We use the smallest allowed window size for receiving the response,
			// and empty request and response messages.
			sendFrames(t, stream,
				&tunnelpb.ClientToServer{
					StreamId: 1,
					Frame: &tunnelpb.ClientToServer_NewStream{
						NewStream: &tunnelpb.NewStream{
							MethodName:        "grpchantesting.TestService/UseExternalMessageTwice",
							ProtocolRevision:  revision,
							InitialWindowSize: minWindowSize(revision),
						},
					},
				},
				&tunnelpb.ClientToServer{
					StreamId: 1,
					Frame:    &tunnelpb.ClientToServer_RequestMessage{RequestMessage: &tunnelpb.MessageData{}},
				},
				&tunnelpb.ClientToServer{
					StreamId: 1,
					Frame:    &tunnelpb.ClientToServer_HalfClose{HalfClose: &emptypb.Empty{}},
				},
			)
			var gotResponse bool
			for {
				in, err := stream.Recv()
				require.NoError(t, err)
				if in.StreamId != 1 {
					continue
				}
				switch frame := in.Frame.(type) {
				case *tunnelpb.ServerToClient_ResponseMessage:
					require.Zero(t, frame.ResponseMessage.Size)
					gotResponse = true
				case *tunnelpb.ServerToClient_CloseStream:
					require.NoError(t, status.FromProto(frame.CloseStream.Status).Err())
					require.True(t, gotResponse, "stream closed without sending response")
					return
				}
			}
		})
	}
}

func TestTunnelServer_RejectsRevisionZeroClientPerStream(t *testing.T) {
	// A client that only supports revision zero (v0.2 or earlier) doesn't
	// send the negotiation header. Such clients don't report the cause when
	// a tunnel fails, so the server instead keeps the tunnel open and rejects
	// each stream, which they do report.
	var svr grpchantesting.TestServer
	tunnelCli, _ := setupInProcessServer(&svr)
	stream, err := tunnelCli.OpenTunnel(t.Context())
	require.NoError(t, err)

	for streamID := int64(1); streamID <= 2; streamID++ {
		// Revision zero clients don't set the protocol revision or window size.
		sendFrames(t, stream,
			&tunnelpb.ClientToServer{
				StreamId: streamID,
				Frame: &tunnelpb.ClientToServer_NewStream{
					NewStream: &tunnelpb.NewStream{MethodName: "grpchantesting.TestService/Unary"},
				},
			},
			&tunnelpb.ClientToServer{
				StreamId: streamID,
				Frame:    &tunnelpb.ClientToServer_RequestMessage{RequestMessage: &tunnelpb.MessageData{}},
			},
			&tunnelpb.ClientToServer{
				StreamId: streamID,
				Frame:    &tunnelpb.ClientToServer_HalfClose{HalfClose: &emptypb.Empty{}},
			},
		)
		in, err := stream.Recv()
		require.NoError(t, err)
		require.Equal(t, streamID, in.StreamId, "server should not send settings to revision zero client")
		closeStream, ok := in.Frame.(*tunnelpb.ServerToClient_CloseStream)
		require.True(t, ok, "expected CloseStream frame, got %T", in.Frame)
		st := status.FromProto(closeStream.CloseStream.Status)
		require.Equal(t, codes.FailedPrecondition, st.Code())
		require.Contains(t, st.Message(), "server does not support protocol revision 0 anymore; upgrade client to v0.3 or later")
	}
}

func TestReverseTunnelServer_RejectsRevisionZeroPeerPerStream(t *testing.T) {
	// In a reverse tunnel, the network server is the tunnel client. So when it
	// only supports revision zero, the error should say to upgrade it.
	var inproc inprocgrpc.Channel
	fakeSvr := &revisionZeroReverseTunnelServer{result: make(chan *status.Status, 1)}
	tunnelpb.RegisterTunnelServiceServer(&inproc, fakeSvr)
	revSvr := NewReverseTunnelServer(tunnelpb.NewTunnelServiceClient(&inproc))
	grpchantesting.RegisterTestServiceServer(revSvr, &grpchantesting.TestServer{})
	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		_, _ = revSvr.Serve(t.Context())
	}()
	defer func() {
		revSvr.Stop()
		<-serveDone
	}()

	select {
	case st := <-fakeSvr.result:
		require.Equal(t, codes.FailedPrecondition, st.Code())
		require.Contains(t, st.Message(), "tunnel server (network client) does not support protocol revision 0 anymore; "+
			"upgrade tunnel client (network server) to v0.3 or later")
	case <-time.After(5 * time.Second):
		t.Fatal("stream was never closed")
	}
}

// revisionZeroReverseTunnelServer acts like a network server from v0.2 or
// earlier, which only supports revision zero. When a reverse tunnel is opened,
// it creates a stream and reports the status with which the stream is closed.
type revisionZeroReverseTunnelServer struct {
	tunnelpb.UnimplementedTunnelServiceServer
	result chan *status.Status
}

func (s *revisionZeroReverseTunnelServer) OpenReverseTunnel(stream tunnelpb.TunnelService_OpenReverseTunnelServer) error {
	// Like v0.2, send headers right away, but without the negotiation header.
	// (v0.2 sends empty headers, but the in-process channel won't actually
	// send headers if there are none, so we send a placeholder.)
	if err := stream.SendHeader(metadata.Pairs("fake-version", "v0.2")); err != nil {
		return err
	}
	// Revision zero peers don't set the protocol revision or window size.
	frames := []*tunnelpb.ClientToServer{
		{
			StreamId: 1,
			Frame: &tunnelpb.ClientToServer_NewStream{
				NewStream: &tunnelpb.NewStream{MethodName: "grpchantesting.TestService/Unary"},
			},
		},
		{
			StreamId: 1,
			Frame:    &tunnelpb.ClientToServer_RequestMessage{RequestMessage: &tunnelpb.MessageData{}},
		},
		{
			StreamId: 1,
			Frame:    &tunnelpb.ClientToServer_HalfClose{HalfClose: &emptypb.Empty{}},
		},
	}
	for _, frame := range frames {
		if err := stream.Send(frame); err != nil {
			return err
		}
	}
	for {
		in, err := stream.Recv()
		if err != nil {
			return err
		}
		if closeStream, ok := in.Frame.(*tunnelpb.ServerToClient_CloseStream); ok && in.StreamId == 1 {
			s.result <- status.FromProto(closeStream.CloseStream.Status)
			return nil
		}
	}
}

func TestTunnelServiceHandler_RejectsRevisionZeroReverseTunnel(t *testing.T) {
	// In a reverse tunnel, the network client is the tunnel server. So when it
	// only supports revision zero, the error should say to upgrade it.
	var svr grpchantesting.TestServer
	tunnelCli, _ := setupInProcessServer(&svr)
	// Like v0.2 and earlier, we don't send the negotiation header.
	stream, err := tunnelCli.OpenReverseTunnel(t.Context())
	require.NoError(t, err)
	_, err = stream.Recv()
	require.ErrorContains(t, err, "tunnel client (network server) does not support protocol revision 0 anymore; "+
		"upgrade tunnel server (network client) to v0.3 or later")
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestTunnelChannel_ClosedChannelError(t *testing.T) {
	var svr grpchantesting.TestServer
	tunnelCli, _ := setupInProcessServer(&svr)
	ch, err := NewChannel(tunnelCli).Start(t.Context())
	require.NoError(t, err)
	ch.Close()
	<-ch.Done()
	// Same as a grpc.ClientConn that has been closed.
	_, err = grpchantesting.NewTestServiceClient(ch).Unary(t.Context(), &grpchantesting.Message{})
	require.Equal(t, codes.Canceled, status.Code(err))
	require.ErrorContains(t, err, "channel is closed")
}

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
			name: "window-too-small-for-revision-two",
			modify: func(ns *tunnelpb.NewStream) {
				ns.ProtocolRevision = tunnelpb.ProtocolRevision_REVISION_TWO
				ns.InitialWindowSize = 5
			},
			expectCode: codes.Internal,
			expectMsg:  "initial window size",
		},
		{
			// like a v0.3 client with flow control disabled
			name:       "revision-zero",
			modify:     func(ns *tunnelpb.NewStream) { ns.ProtocolRevision = tunnelpb.ProtocolRevision_REVISION_ZERO },
			expectCode: codes.FailedPrecondition,
			expectMsg:  "server does not support protocol revision 0 anymore; client must not disable flow control",
		},
		{
			name:       "unknown-revision",
			modify:     func(ns *tunnelpb.NewStream) { ns.ProtocolRevision = 99 },
			expectCode: codes.FailedPrecondition,
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

func TestTunnelChannel_NegotiatesRevision(t *testing.T) {
	revisions := func(revs ...tunnelpb.ProtocolRevision) []tunnelpb.ProtocolRevision { return revs }
	const (
		zero = tunnelpb.ProtocolRevision_REVISION_ZERO
		one  = tunnelpb.ProtocolRevision_REVISION_ONE
		two  = tunnelpb.ProtocolRevision_REVISION_TWO
	)
	testCases := []struct {
		name string
		// if true, the server acts like v0.2 and earlier, which don't negotiate
		revisionZero bool
		revisions    []tunnelpb.ProtocolRevision
		windowSize   uint32
		// either the revision the client should use or the error it should report
		expectRevision tunnelpb.ProtocolRevision
		expectErr      string
		expectCode     codes.Code
	}{
		{
			// like a v0.3 server
			name:           "zero-and-one",
			revisions:      revisions(zero, one),
			windowSize:     defaultInitialWindowSize,
			expectRevision: one,
		},
		{
			name:           "one-and-two",
			revisions:      revisions(one, two),
			windowSize:     defaultInitialWindowSize,
			expectRevision: two,
		},
		{
			name:           "includes-unknown-revision",
			revisions:      revisions(one, two, 99),
			windowSize:     defaultInitialWindowSize,
			expectRevision: two,
		},
		{
			// like a v0.2 server, which doesn't negotiate
			name:         "no-negotiation",
			revisionZero: true,
			expectErr:    "client does not support protocol revision 0 anymore; upgrade server to v0.3 or later",
			expectCode:   codes.FailedPrecondition,
		},
		{
			// like a v0.3 server with flow control disabled
			name:       "only-zero",
			revisions:  revisions(zero),
			windowSize: defaultInitialWindowSize,
			expectErr:  "client does not support protocol revision 0 anymore; server must not disable flow control",
			expectCode: codes.FailedPrecondition,
		},
		{
			name:       "zero-window",
			revisions:  revisions(one),
			windowSize: 0,
			expectErr:  "initial window size",
			expectCode: codes.Internal,
		},
		{
			name:           "small-window-ok-for-revision-one",
			revisions:      revisions(one),
			windowSize:     1,
			expectRevision: one,
		},
		{
			name:       "window-too-small-for-revision-two",
			revisions:  revisions(one, two),
			windowSize: 5,
			expectErr:  "initial window size",
			expectCode: codes.Internal,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fakeSvr := &fakeTunnelServer{
				revisionZero: testCase.revisionZero,
				settings: &tunnelpb.Settings{
					SupportedProtocolRevisions: testCase.revisions,
					InitialWindowSize:          testCase.windowSize,
				},
				newStreams: make(chan *tunnelpb.NewStream, 1),
			}
			var inproc inprocgrpc.Channel
			tunnelpb.RegisterTunnelServiceServer(&inproc, fakeSvr)
			ch, err := NewChannel(tunnelpb.NewTunnelServiceClient(&inproc)).Start(t.Context())
			require.NoError(t, err)
			defer ch.Close()

			if testCase.expectErr != "" {
				select {
				case <-ch.Done():
				case <-time.After(5 * time.Second):
					t.Fatal("tunnel was not closed after receiving bad settings")
				}
				require.ErrorContains(t, ch.Err(), testCase.expectErr)
				require.Equal(t, testCase.expectCode, status.Code(ch.Err()))
				// RPCs should also report the reason the tunnel was closed.
				_, err := grpchantesting.NewTestServiceClient(ch).Unary(t.Context(), &grpchantesting.Message{})
				require.ErrorContains(t, err, testCase.expectErr)
				require.Equal(t, testCase.expectCode, status.Code(err))
				return
			}

			// Start an RPC, so we can see what revision the client uses. The fake
			// server never replies, so we cancel the RPC when done.
			ctx, cancel := context.WithCancel(t.Context())
			rpcDone := make(chan struct{})
			go func() {
				defer close(rpcDone)
				_, _ = grpchantesting.NewTestServiceClient(ch).Unary(ctx, &grpchantesting.Message{})
			}()
			defer func() {
				cancel()
				<-rpcDone
			}()
			select {
			case newStream := <-fakeSvr.newStreams:
				require.Equal(t, testCase.expectRevision, newStream.ProtocolRevision)
			case <-time.After(5 * time.Second):
				t.Fatalf("client never created stream; tunnel error: %v", ch.Err())
			}
		})
	}
}

// fakeTunnelServer is a tunnel server that sends the given settings and then
// reports any NewStream frames it receives, without ever replying to them.
// If revisionZero is true, it instead acts like v0.2, which doesn't negotiate
// or send settings.
type fakeTunnelServer struct {
	tunnelpb.UnimplementedTunnelServiceServer
	revisionZero bool
	settings     *tunnelpb.Settings
	newStreams   chan *tunnelpb.NewStream
}

func (s *fakeTunnelServer) OpenTunnel(stream tunnelpb.TunnelService_OpenTunnelServer) error {
	if s.revisionZero {
		// Like v0.2, send headers right away, but without the negotiation header.
		// (v0.2 sends empty headers, but the in-process channel won't actually
		// send headers if there are none, so we send a placeholder.)
		if err := stream.SendHeader(metadata.Pairs("fake-version", "v0.2")); err != nil {
			return err
		}
	} else {
		if err := stream.SendHeader(metadata.Pairs(grpctunnelNegotiateKey, grpctunnelNegotiateVal)); err != nil {
			return err
		}
		err := stream.Send(&tunnelpb.ServerToClient{
			StreamId: -1,
			Frame:    &tunnelpb.ServerToClient_Settings{Settings: s.settings},
		})
		if err != nil {
			return err
		}
	}
	for {
		in, err := stream.Recv()
		if err != nil {
			// client hung up
			return nil
		}
		if newStream, ok := in.Frame.(*tunnelpb.ClientToServer_NewStream); ok {
			select {
			case s.newStreams <- newStream.NewStream:
			default:
			}
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
