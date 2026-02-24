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
)

const (
	defaultInitialWindowSize = 65536
	defaultChunkMax          = 16384
)

var errFlowControlWindowExceeded = status.Errorf(codes.ResourceExhausted, "flow control window exceeded")

// sender is responsible for sending messages and managing flow control.
// When sending data, it will not send more bytes than allowed by the current
// flow control window. If more is to be sent, the operation will block until
// the receiver acknowledges data and adds more capacity to the flow control
// window.
type sender struct {
	ctx           context.Context
	maxChunkSize  uint32
	sendFunc      func([]byte, uint32, bool) error
	windowUpdates chan struct{}
	currentWindow atomic.Uint32

	// does not protect any fields, just used to prevent concurrent calls to send
	// (so messages are sent FIFO and not incorrectly interleaved)
	mu sync.Mutex
}

func newSender(ctx context.Context, initialWindowSize, maxChunkSize uint32, sendFunc func([]byte, uint32, bool) error) *sender {
	s := &sender{
		ctx:           ctx,
		maxChunkSize:  maxChunkSize,
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
	if prevWindow == 0 {
		// Window changed from zero to non-zero, so unblock any sender
		// that was waiting to send more data.
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
	first := true
	for {
		windowSz := s.currentWindow.Load()

		if windowSz == 0 {
			// must wait for window size update before we can send more
			select {
			case <-s.windowUpdates:
			case <-s.ctx.Done():
				return s.ctx.Err()
			}
			continue
		}

		chunkSz := windowSz
		if chunkSz > uint32(len(data)) {
			chunkSz = uint32(len(data))
		}
		if chunkSz > s.maxChunkSize {
			chunkSz = s.maxChunkSize
		}
		if !s.currentWindow.CompareAndSwap(windowSz, windowSz-chunkSz) {
			continue
		}

		last := chunkSz == uint32(len(data))
		if err := s.sendFunc(data[:chunkSz], size, first); err != nil {
			return err
		}
		if last {
			return nil
		}
		first = false

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
	measure      func(T) uint
	updateWindow func(uint32)

	mu                sync.Mutex
	cond              sync.Cond
	closed, cancelled bool
	items             *list.List
	currentWindow     uint32
}

func newReceiver[T any](measure func(T) uint, updateWindow func(uint32), initialWindowSize uint32) *receiver[T] {
	rcvr := &receiver[T]{
		measure:       measure,
		updateWindow:  updateWindow,
		items:         list.New(),
		currentWindow: initialWindowSize,
	}
	rcvr.cond.L = &rcvr.mu
	return rcvr
}

func (r *receiver[T]) accept(item T) error {
	sz := r.measure(item)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
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
	var windowUpdate uint
	defer func() {
		// TODO: Support minimum update size, so we can batch
		//       updates and send fewer messages over the network.
		if windowUpdate > 0 {
			r.updateWindow(uint32(windowUpdate))
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
			sz := r.measure(item)
			r.currentWindow += uint32(sz)
			windowUpdate = sz
			return item, true
		}
		if r.closed {
			return zero, false
		}
		r.cond.Wait()
	}
}
