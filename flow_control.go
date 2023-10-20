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
	initialWindowSize = 65536
	chunkMax          = 16384
)

var errFlowControlWindowExceeded = status.Errorf(codes.ResourceExhausted, "flow control window exceeded")

// sender is responsible for sending messages and managing flow control.
type sender interface {
	send(data []byte) error
	updateWindow(add uint32)
}

// receiver is responsible for receiving messages and managing flow control.
type receiver[T any] interface {
	accept(item T) error
	close()
	cancel()
	dequeue() (T, bool)
}

type defaultSender struct {
	ctx           context.Context
	sendFunc      func([]byte, uint32, bool) error
	windowUpdates chan struct{}
	currentWindow atomic.Uint32

	// does not protect any fields, just used to prevent concurrent calls to send
	// (so messages are sent FIFO and not incorrectly interleaved)
	mu sync.Mutex
}

func newSender(ctx context.Context, initialWindowSize uint32, sendFunc func([]byte, uint32, bool) error) sender {
	s := &defaultSender{
		ctx:           ctx,
		sendFunc:      sendFunc,
		windowUpdates: make(chan struct{}, 1),
	}
	s.currentWindow.Store(initialWindowSize)
	return s
}

func (s *defaultSender) updateWindow(add uint32) {
	if add == 0 {
		return
	}
	prevWindow := s.currentWindow.Add(add) - add
	if prevWindow == 0 {
		select {
		case s.windowUpdates <- struct{}{}:
		default:
		}
	}
}

func (s *defaultSender) send(data []byte) error {
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
		if chunkSz > chunkMax {
			chunkSz = chunkMax
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

// defaultReceiver is a per-stream queue of messages. When we receive a message for
// a stream over a tunnel, we have to put them into this unbounded queue to prevent
// deadlock (where one consumer of a stream channel can block all operations on the
// tunnel).
//
// In practice, this does not use unbounded memory because flow control will apply
// backpressure to senders that are outpacing respective consumers. A well-behaved
// sender will respect the flow control window. A misbehaving sender will be detected
// and messages rejected if the flow control window is exceeded.
type defaultReceiver[T any] struct {
	measure      func(T) uint
	updateWindow func(uint32)

	mu                sync.Mutex
	cond              sync.Cond
	closed, cancelled bool
	items             *list.List
	currentWindow     uint32
}

func newReceiver[T any](measure func(T) uint, updateWindow func(uint32), initialWindowSize uint32) receiver[T] {
	rcvr := &defaultReceiver[T]{
		measure:       measure,
		updateWindow:  updateWindow,
		items:         list.New(),
		currentWindow: initialWindowSize,
	}
	rcvr.cond.L = &rcvr.mu
	return rcvr
}

func (r *defaultReceiver[T]) accept(item T) error {
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

func (r *defaultReceiver[_]) close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handleClosure(&r.closed)
}

func (r *defaultReceiver[_]) cancel() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handleClosure(&r.cancelled)
	r.items.Init() // clear list to free memory
}

func (r *defaultReceiver[_]) handleClosure(b *bool) {
	if *b {
		return
	}
	*b = true
	if r.items.Len() == 0 {
		r.cond.Broadcast()
	}
}

func (r *defaultReceiver[T]) dequeue() (T, bool) {
	var windowUpdate uint
	defer func() {
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

type noFlowControlSender struct {
	sendFunc func([]byte, uint32, bool) error

	// does not protect any fields, just used to prevent concurrent calls to send
	// (so messages are sent FIFO and not incorrectly interleaved)
	mu sync.Mutex
}

func newSenderWithoutFlowControl(sendFunc func([]byte, uint32, bool) error) sender {
	return &noFlowControlSender{sendFunc: sendFunc}
}

func (s *noFlowControlSender) send(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if int64(len(data)) > math.MaxUint32 {
		return status.Errorf(codes.ResourceExhausted, "serialized message is too large: %d bytes > maximum %d bytes", len(data), math.MaxUint32)
	}
	size := uint32(len(data))
	first := true
	for {
		chunkSz := uint32(chunkMax)
		if chunkSz > uint32(len(data)) {
			chunkSz = uint32(len(data))
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

func (s *noFlowControlSender) updateWindow(_ uint32) {
	// should never actually be called
}

type noFlowControlReceiver[T any] struct {
	ctx context.Context

	ch      chan T
	closed  chan struct{}
	doClose sync.Once
}

func newReceiverWithoutFlowControl[T any](ctx context.Context) receiver[T] {
	return &noFlowControlReceiver[T]{
		ctx:    ctx,
		ch:     make(chan T, 1),
		closed: make(chan struct{}),
	}
}

func (r *noFlowControlReceiver[T]) accept(item T) error {
	// If closed is already done, don't try to add an item to ch
	select {
	case <-r.closed:
		return nil
	default:
	}
	select {
	case r.ch <- item:
	case <-r.closed:
	}
	return nil
}

func (r *noFlowControlReceiver[T]) close() {
	r.doClose.Do(func() {
		close(r.closed)
	})
}

func (r *noFlowControlReceiver[T]) cancel() {
	r.close()
}

func (r *noFlowControlReceiver[T]) dequeue() (T, bool) {
	var zero T
	if r.ctx.Err() != nil {
		return zero, false
	}
	// If there's an item in ch, make sure to use it
	// before potentially looking at closed channel
	select {
	case t, ok := <-r.ch:
		return t, ok
	default:
	}
	select {
	case t, ok := <-r.ch:
		return t, ok
	case <-r.closed:
		return zero, false
	case <-r.ctx.Done():
		return zero, false
	}
}
