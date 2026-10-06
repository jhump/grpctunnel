package grpctunnel

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"
)

// testFrame is a stand-in for a data frame, for testing senders and receivers
// without a tunnel.
type testFrame struct {
	size     uint
	msgStart bool
}

func measureTestFrame(frame testFrame) (uint, bool) {
	return frame.size, frame.msgStart
}

// chunkRecorder records the chunks sent by a sender.
type chunkRecorder struct {
	mu     sync.Mutex
	chunks []testFrame
}

func (r *chunkRecorder) send(data []byte, _ uint32, first bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.chunks = append(r.chunks, testFrame{size: uint(len(data)), msgStart: first})
	return nil
}

func (r *chunkRecorder) get() []testFrame {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]testFrame(nil), r.chunks...)
}

func TestSender_EmptyMessages(t *testing.T) {
	testCases := []struct {
		name        string
		overhead    uint32
		expectCount int32
	}{
		{
			// Empty messages consume no window, so all of them are sent.
			name:        "no-overhead",
			overhead:    0,
			expectCount: 1000,
		},
		{
			// Each empty message consumes the overhead, so the sender blocks
			// once the window is exhausted.
			name:        "overhead",
			overhead:    revisionTwoMessageOverhead,
			expectCount: 1024 / revisionTwoMessageOverhead,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				var count atomic.Int32
				s := newSender(ctx, 1024, defaultChunkMax, testCase.overhead,
					func([]byte, uint32, bool) error {
						count.Add(1)
						return nil
					},
					"test")
				done := make(chan struct{})
				go func() {
					defer close(done)
					for range 1000 {
						if err := s.send(nil); err != nil {
							return
						}
					}
				}()
				synctest.Wait()
				require.Equal(t, testCase.expectCount, count.Load())
				cancel()
				<-done
			})
		})
	}
}

func TestSender_WaitsForRoomForOverhead(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var recorder chunkRecorder
		s := newSender(t.Context(), 1024, defaultChunkMax, revisionTwoMessageOverhead, recorder.send, "test")

		// This consumes the whole window: 5 bytes of overhead + 1019 bytes of data.
		require.NoError(t, s.send(make([]byte, 1019)))
		require.Equal(t, []testFrame{{size: 1019, msgStart: true}}, recorder.get())

		done := make(chan struct{})
		go func() {
			defer close(done)
			require.NoError(t, s.send([]byte{1}))
		}()
		synctest.Wait()
		require.Len(t, recorder.get(), 1, "sender should be blocked with empty window")

		// Five bytes is enough for the overhead, but not for any data.
		s.updateWindow(5)
		synctest.Wait()
		require.Len(t, recorder.get(), 1, "sender should be blocked without room for overhead plus data")

		s.updateWindow(1)
		<-done
		require.Equal(t, []testFrame{{size: 1019, msgStart: true}, {size: 1, msgStart: true}}, recorder.get())
	})
}

func TestSender_OverheadOnlyChargedForFirstChunk(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var recorder chunkRecorder
		s := newSender(t.Context(), 1024, defaultChunkMax, revisionTwoMessageOverhead, recorder.send, "test")

		done := make(chan struct{})
		go func() {
			defer close(done)
			require.NoError(t, s.send(make([]byte, 1100)))
		}()
		synctest.Wait()
		require.Equal(t, []testFrame{{size: 1019, msgStart: true}}, recorder.get())

		// Subsequent chunks only need room for data.
		s.updateWindow(1)
		synctest.Wait()
		require.Equal(t, []testFrame{{size: 1019, msgStart: true}, {size: 1}}, recorder.get())

		s.updateWindow(1024)
		<-done
		require.Equal(t, []testFrame{{size: 1019, msgStart: true}, {size: 1}, {size: 80}}, recorder.get())
	})
}

func TestReceiver_ChargesOverhead(t *testing.T) {
	testCases := []struct {
		name     string
		overhead uint32
		// number of empty messages that can be accepted before exceeding the window
		// (-1 means unlimited)
		expectAccepted int
	}{
		{
			name:           "no-overhead",
			overhead:       0,
			expectAccepted: -1,
		},
		{
			name:           "overhead",
			overhead:       revisionTwoMessageOverhead,
			expectAccepted: 1024 / revisionTwoMessageOverhead,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			r := newReceiver(measureTestFrame, func(uint32) {}, 1024, 1024, testCase.overhead, "test")
			emptyMessage := testFrame{msgStart: true}
			if testCase.expectAccepted < 0 {
				// A misbehaving sender can't exceed the window with empty messages.
				for range 10_000 {
					require.NoError(t, r.accept(emptyMessage))
				}
				return
			}
			for range testCase.expectAccepted {
				require.NoError(t, r.accept(emptyMessage))
			}
			require.ErrorIs(t, r.accept(emptyMessage), errFlowControlWindowExceeded)
			// Data in subsequent chunks is not charged the overhead.
			remaining := 1024 - uint(testCase.expectAccepted)*revisionTwoMessageOverhead
			require.NoError(t, r.accept(testFrame{size: remaining}))
			require.ErrorIs(t, r.accept(testFrame{size: 1}), errFlowControlWindowExceeded)
		})
	}
}

func TestFlowControl_NoStallWithLargeMinUpdateSize(t *testing.T) {
	// The minimum update size is the whole window, so the receiver normally
	// only sends a window update after the entire window is consumed. But with
	// per-message overhead, the sender can be blocked before that happens: it
	// can't send the next message until it has room for the overhead plus data.
	testCases := []struct {
		name     string
		overhead uint32
	}{
		{
			name:     "no-overhead",
			overhead: 0,
		},
		{
			name:     "overhead",
			overhead: revisionTwoMessageOverhead,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var s *sender
				r := newReceiver(measureTestFrame, func(update uint32) { s.updateWindow(update) },
					1024, 1024, testCase.overhead, "test")
				s = newSender(t.Context(), 1024, defaultChunkMax, testCase.overhead,
					func(data []byte, _ uint32, first bool) error {
						return r.accept(testFrame{size: uint(len(data)), msgStart: first})
					},
					"test")
				consumerDone := make(chan struct{})
				go func() {
					defer close(consumerDone)
					for {
						if _, ok := r.dequeue(); !ok {
							return
						}
					}
				}()

				// Leave three bytes in the window.
				require.NoError(t, s.send(make([]byte, 1024-testCase.overhead-3)))
				// If the receiver doesn't acknowledge what it has consumed, this
				// will block forever (and synctest will report a deadlock).
				require.NoError(t, s.send([]byte{1}))

				r.close()
				<-consumerDone
			})
		})
	}
}
