package grpctunnel

//lint:file-ignore U1000 these aren't actually unused, but staticcheck is having trouble
//                       determining that, likely due to the use of generics

import (
	"container/list"
	"context"
	"math"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/jhump/grpctunnel/tunnelpb"
)

const (
	defaultInitialWindowSize = 64 * 1024
	minInitialWindowSize     = 1024
	defaultChunkMax          = 16 * 1024
	defaultUpdateMin         = 16 * 1024

	// revisionTwoMessageOverhead is the number of bytes charged against the
	// flow control window for each message, in addition to the message data,
	// starting with revision two of the protocol.
	revisionTwoMessageOverhead = 5
)

// messageOverhead returns the number of bytes charged against the flow control
// window for each message, in addition to the message data, for the given
// protocol revision. In revision one, only message data is charged, so empty
// messages are not subject to flow control.
//
// This, along with minWindowSize, is how the protocol revision of a stream is
// translated into flow control behavior. Senders and receivers only look at the
// resulting overhead, not at the revision.
func messageOverhead(revision tunnelpb.ProtocolRevision) uint32 {
	if revision >= tunnelpb.ProtocolRevision_REVISION_TWO {
		return revisionTwoMessageOverhead
	}
	return 0
}

// minWindowSize returns the smallest valid initial window size for the given
// protocol revision. The window must have room for the overhead of a message
// plus at least one byte of data.
func minWindowSize(revision tunnelpb.ProtocolRevision) uint32 {
	return messageOverhead(revision) + 1
}

var errFlowControlWindowExceeded = status.Errorf(codes.ResourceExhausted, "flow control window exceeded")

// sender is responsible for sending messages and managing flow control.
// When sending data, it will not send more bytes than allowed by the current
// flow control window. If more is to be sent, the operation will block until
// the receiver acknowledges data and adds more capacity to the flow control
// window.
type sender struct {
	ctx           context.Context
	streamName    string
	maxChunkSize  uint32
	msgOverhead   uint32
	sendFunc      func([]byte, uint32, bool) error
	windowUpdates chan struct{}
	currentWindow atomic.Uint32

	// does not protect any fields, just used to prevent concurrent calls to send
	// (so messages are sent FIFO and not incorrectly interleaved)
	mu sync.Mutex
}

func newSender(
	ctx context.Context,
	initialWindowSize, maxChunkSize, msgOverhead uint32,
	sendFunc func([]byte, uint32, bool) error,
	streamName string,
) *sender {
	if maxChunkSize > initialWindowSize {
		maxChunkSize = initialWindowSize
	}
	s := &sender{
		ctx:           ctx,
		streamName:    streamName,
		maxChunkSize:  maxChunkSize,
		msgOverhead:   msgOverhead,
		sendFunc:      sendFunc,
		windowUpdates: make(chan struct{}, 1),
	}
	s.currentWindow.Store(initialWindowSize)
	return s
}

func (s *sender) updateWindow(add uint32) {
	if add == 0 {
		return
	}
	prevWindow := s.currentWindow.Add(add) - add
	logSenderUpdate(s.streamName, add, prevWindow)
	if prevWindow <= s.msgOverhead {
		// The window may have been too small to send anything, so unblock
		// any sender that was waiting to send more data.
		select {
		case s.windowUpdates <- struct{}{}:
		default:
		}
	}
}

func (s *sender) send(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if int64(len(data)) > math.MaxUint32 {
		return status.Errorf(codes.ResourceExhausted, "serialized message is too large: %d bytes > maximum %d bytes", len(data), math.MaxUint32)
	}
	size := uint32(len(data))
	chunkIndex := 0
	for {
		windowSz := s.currentWindow.Load()

		// The first chunk of a message is also charged the per-message overhead.
		var overhead uint32
		if chunkIndex == 0 {
			overhead = s.msgOverhead
		}
		// We need room for the overhead plus at least one byte of data (just the
		// overhead if the message is empty). But we always need at least one byte.
		needed := max(overhead+min(uint32(len(data)), 1), 1)
		if windowSz < needed {
			// must wait for window size update before we can send more
			select {
			case <-s.windowUpdates:
			case <-s.ctx.Done():
				return s.ctx.Err()
			}
			continue
		}

		chunkSz := min(windowSz-overhead, uint32(len(data)), s.maxChunkSize)
		if !s.currentWindow.CompareAndSwap(windowSz, windowSz-overhead-chunkSz) {
			continue
		}
		logSend(s.streamName, chunkIndex, chunkSz, overhead, uint32(len(data)), windowSz)

		last := chunkSz == uint32(len(data))
		if err := s.sendFunc(data[:chunkSz], size, chunkIndex == 0); err != nil {
			return err
		}
		if last {
			return nil
		}
		chunkIndex++

		data = data[chunkSz:]
	}
}

// receiver is responsible for receiving messages and managing flow control. It
// holds a per-stream queue of messages. When we receive a message for a stream
// over a tunnel, we have to put them into this unbounded queue to prevent
// deadlock (where one consumer of a stream channel can block all operations on the
// tunnel).
//
// In practice, this does not use unbounded memory because flow control will apply
// backpressure to senders that are outpacing respective consumers. A well-behaved
// sender will respect the flow control window. A misbehaving sender will be detected
// and messages rejected if the flow control window is exceeded.
type receiver[T any] struct {
	streamName string
	// measure returns the size of the data in the given item and whether the
	// item is the first chunk of a message (so is also charged the overhead).
	measure       func(T) (size uint, msgStart bool)
	updateWindow  func(uint32)
	minUpdateSize uint32
	msgOverhead   uint32

	mu                sync.Mutex
	cond              sync.Cond
	closed, cancelled bool
	items             *list.List
	currentWindow     uint32
	windowUpdate      uint32
}

func newReceiver[T any](
	measure func(T) (size uint, msgStart bool),
	updateWindow func(uint32),
	initialWindowSize, minUpdateSize, msgOverhead uint32,
	streamName string,
) *receiver[T] {
	if minUpdateSize > initialWindowSize {
		minUpdateSize = initialWindowSize
	}
	rcvr := &receiver[T]{
		streamName:    streamName,
		measure:       measure,
		updateWindow:  updateWindow,
		minUpdateSize: minUpdateSize,
		msgOverhead:   msgOverhead,
		items:         list.New(),
		currentWindow: initialWindowSize,
	}
	rcvr.cond.L = &rcvr.mu
	return rcvr
}

// charge returns the number of bytes of the flow control window that the given
// item consumes.
func (r *receiver[T]) charge(item T) uint {
	sz, msgStart := r.measure(item)
	if msgStart {
		sz += uint(r.msgOverhead)
	}
	return sz
}

func (r *receiver[T]) accept(item T) error {
	sz := r.charge(item)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	logReceive(r.streamName, sz, r.currentWindow)
	if sz > uint(r.currentWindow) {
		return errFlowControlWindowExceeded
	}
	r.currentWindow -= uint32(sz)
	signal := r.items.Len() == 0
	r.items.PushBack(item)
	if signal {
		r.cond.Signal()
	}
	return nil
}

func (r *receiver[_]) close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handleClosure(&r.closed)
}

func (r *receiver[_]) cancel() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handleClosure(&r.cancelled)
	r.items.Init() // clear list to free memory
}

func (r *receiver[_]) handleClosure(b *bool) {
	if *b {
		return
	}
	*b = true
	if r.items.Len() == 0 {
		r.cond.Broadcast()
	}
}

func (r *receiver[T]) dequeue() (T, bool) {
	var sendUpdate uint32
	defer func() {
		if sendUpdate > 0 {
			r.updateWindow(sendUpdate)
		}
	}()
	r.mu.Lock()
	defer r.mu.Unlock()
	var zero T
	for {
		if r.cancelled {
			return zero, false
		}
		element := r.items.Front()
		if element != nil {
			item := r.items.Remove(element).(T)
			sz := r.charge(item)
			newWindowUpdate := r.windowUpdate + uint32(sz)
			doSend := newWindowUpdate >= r.minUpdateSize
			if !doSend && r.msgOverhead > 0 && newWindowUpdate > 0 && r.currentWindow <= r.msgOverhead {
				// The sender's window may be too small to send the next message,
				// so we can't wait until we have minUpdateSize to acknowledge.
				// (When there is no message overhead, the sender only stalls when
				// its window is zero, at which point we will have acknowledged
				// the entire initial window, which is at least minUpdateSize.)
				doSend = true
			}
			logReceiverAck(r.streamName, sz, newWindowUpdate, r.currentWindow, doSend)
			if doSend {
				r.currentWindow += newWindowUpdate
				sendUpdate = newWindowUpdate
				newWindowUpdate = 0
			}
			r.windowUpdate = newWindowUpdate
			return item, true
		}
		if r.closed {
			return zero, false
		}
		r.cond.Wait()
	}
}
