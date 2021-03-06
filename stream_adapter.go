package grpctunnel

import (
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"io"
	"reflect"
	"sync"
	"sync/atomic"
)

// CorruptResponseStreamError is an error that occurs when a correlated stream
// adapter receives a message that cannot be correlated to a pending request.
type CorruptResponseStreamError struct {
	resp  interface{}
	key   interface{}
	count uint64
}

// Error implements the error interface
func (e CorruptResponseStreamError) Error() string {
	return fmt.Sprintf("Corrupt response stream: received message #%d for unexpected key %v: %v", e.count, e.key, e.resp)
}

// response is a tuple that can hold either a response message or an error
type response struct {
	resp interface{}
	err  error
}

// NewStreamAdapter returns a stream adapter that correlates responses with
// their corresponding requests via the given key extractor functions. If either
// function is nil, both must be nil (in which case the extracted key is the nil
// interface, meaning everything must be correlated via FIFO order).
//
// A request message is supplied to requestKeyExtractor to determine the key.
// When a response message is received, the given responseKeyExtractor is
// consulted. The resulting response key is expected to match the key of a
// pending request. If a message is received that does not correspond to any
// pending request, the stream is closed and all pending operations fail with
// a CorruptResponseStreamError.
//
// If there are multiple pending requests for a particular key, they are
// serviced in FIFO order. So if two requests map to the same key, the first
// matching response is assumed to correspond to the first such request, and
// so on.
//
// The given responseFactory is used to create new instances into which response
// data is unmarshaled. If it is nil, the given stream must have a Recv method
// that accepts no arguments and returns two: a response message and an error.
// The return type will be examined (via reflection) and values of the right
// type will be created via reflection. Stream types generated by protoc for use
// with normal gRPC streaming stubs will have such a method and thus can be used
// so that calling code need not explicitly supply a factory.
//
// This function is used on the client side of the stream -- the side that will
// send requests and then expects responses in reply. (This does not necessarily
// have to be a gRPC client though, as a stream could allow servers to initiate
// requests this way). For the server side of the stream, see HandleServerStream.
func NewStreamAdapter(stream grpc.Stream, requestKeyExtractor, responseKeyExtractor func(interface{}) interface{}, responseFactory func() interface{}) *StreamAdapter {
	if (requestKeyExtractor == nil) == (responseKeyExtractor == nil) {
		panic("if requestKeyExtractor and responseKeyExtractor must either both nil or both be non-nil")
	}
	if responseFactory == nil {
		responseFactory = createFactory(stream)
	}

	ctx, cancel := context.WithCancel(stream.Context())
	return &StreamAdapter{
		ctx:                  ctx,
		cancel:               cancel,
		stream:               stream,
		requestKeyExtractor:  requestKeyExtractor,
		responseKeyExtractor: responseKeyExtractor,
		responseFactory:      responseFactory,
		pending:              map[interface{}][]chan<- response{},
	}
}

var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

func createFactory(stream grpc.Stream) func() interface{} {
	t := reflect.TypeOf(stream)
	mtd, ok := t.MethodByName("Recv")
	if !ok {
		panic("stream has no Recv method from which type can be inferred")
	}
	mt := mtd.Type
	if mt.NumIn() != 1 || mt.In(0) != t {
		panic(fmt.Sprintf("stream's Recv method has wrong signature: %v", mt))
	}
	if mt.NumOut() != 2 || mt.Out(1) != typeOfError {
		panic(fmt.Sprintf("stream's Recv method has wrong signature: %v", mt))
	}
	respType := mt.Out(0)
	if respType.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("stream's Recv method has wrong signature: %v", mt))
	}

	return func() interface{} {
		return reflect.New(respType.Elem()).Interface()
	}
}

// StreamAdapter wraps a grpc.Stream and implements a simple request-response
// mechanism on top of it.
type StreamAdapter struct {
	ctx                  context.Context
	cancel               context.CancelFunc
	stream               grpc.Stream
	responseFactory      func() interface{}
	requestKeyExtractor  func(interface{}) interface{}
	responseKeyExtractor func(interface{}) interface{}

	messageCount uint64

	mu      sync.Mutex
	failure error
	pending map[interface{}][]chan<- response
}

// Call performs a single request-response round trip. It sends the given
// request on the stream and waits for a corresponding response, which is
// recorded in the given response message. It returns a non-nil error in
// the event of a failure.
//
// If the given context contains request metadata, they are ignored. Also,
// as observed in the signature, gRPC call options are not supported. The
// context is only used to allow the call to return early in the event of
// context completion (in which case the return values will be the context
// error). If the context times out or is cancelled, nothing happens in the
// underlying stream, and any response message that eventually arrives will
// effectively be ignored.
func (a *StreamAdapter) Call(ctx context.Context, req interface{}) (interface{}, error) {
	// optimistically add response acceptor to pending set
	var key interface{}
	if a.requestKeyExtractor != nil {
		key = a.requestKeyExtractor(req)
	}
	ch := make(chan response, 1)

	err, startRecvLoop := a.addPending(key, ch)
	if err != nil {
		return nil, err
	}

	if err := a.stream.SendMsg(req); err != nil {
		// remove from pending set
		if !a.removePending(key, ch) {
			// it must have been concurrently removed by recvLoop on stream failure
			select {
			case r := <-ch:
				return r.resp, r.err
			default:
				// TODO: shouldn't ever happen... panic?
			}
		}
		return nil, err
	}

	// make sure we have something accepting response messages
	if startRecvLoop {
		go a.recvLoop()
	}

	// wait for response
	select {
	case r := <-ch:
		return r.resp, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (a *StreamAdapter) addPending(key interface{}, ch chan<- response) (error, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.failure != nil {
		return a.failure, false
	}
	shouldStartRecvLoop := len(a.pending) == 0
	a.pending[key] = append(a.pending[key], ch)
	return nil, shouldStartRecvLoop
}

func (a *StreamAdapter) removePending(key interface{}, ch chan<- response) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	resps := a.pending[key]
	for i, v := range resps {
		if v == ch {
			// remove element from slice
			copy(resps[i:], resps[i+1:])
			a.pending[key] = resps[:len(resps)-1]
			return true
		}
	}

	return false
}

func (a *StreamAdapter) Context() context.Context {
	return a.ctx
}

func (a *StreamAdapter) recvLoop() {
	for {
		m := a.responseFactory()
		if err := a.stream.RecvMsg(m); err != nil {
			// fail all pending operations and bail
			a.abortPending(err)
			return
		}
		c := atomic.AddUint64(&a.messageCount, 1)
		var key interface{}
		if a.responseKeyExtractor != nil {
			key = a.responseKeyExtractor(m)
		}
		done := a.replyToPending(m, key, c)
		if done {
			return
		}
	}
}

func (a *StreamAdapter) replyToPending(resp, key interface{}, count uint64) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	noMorePending := false

	var ch chan<- response
	if chans, ok := a.pending[key]; ok {
		ch = chans[0]
		if len(chans) > 1 {
			a.pending[key] = chans[1:]
		} else {
			delete(a.pending, key)
			if len(a.pending) == 0 {
				noMorePending = true
			}
		}
		ch <- response{resp, nil}
		close(ch)
	} else {
		// no pending request that corresponds to the received response?
		if cs, ok := a.stream.(grpc.ClientStream); ok {
			cs.CloseSend()
		}
		a.cancel()

		a.failure = CorruptResponseStreamError{resp, key, count}
		a.abortPendingLocked(a.failure)
		noMorePending = true
	}

	return noMorePending
}

func (a *StreamAdapter) abortPending(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.abortPendingLocked(err)
}

func (a *StreamAdapter) abortPendingLocked(err error) {
	for _, chans := range a.pending {
		for _, ch := range chans {
			ch <- response{nil, err}
			close(ch)
		}
	}
	a.pending = map[interface{}][]chan<- response{}
}

var typeOfContext = reflect.TypeOf((*context.Context)(nil)).Elem()

// HandleServerStream reads request messages from the given stream, dispatching
// them to the given serveFunc. The serveFunc must be a function that either
// accepts one argument and returns one value or accepts two arguments, the
// first of which must be a context.Context, and returns one value.
//
// The given requestKeyExtractor is used to determine a key. All requests for
// the same key are processed in FIFO order. If no such extractor is given, then
// the nil interface is the assumed key, eliminating parallelism. If an
// extractor is given, requests for different keys will be dispatched to
// serveFunc concurrently, from different goroutines. It is serveFunc's job to
// mark its return values with the same key, so that the client can correctly
// correlate responses with outstanding RPC requests.
//
// The given requestFactory is used to create request instances, into which
// stream messages are unmarshaled. If nil is supplied, a factory will be
// created by querying the given stream for a method named "Recv" (via
// reflection). If it has such a method, it should take no arguments and return
// a value and an error. The type of that first return value will be used to
// reflectively construct request instances.
//
// This function returns an error if a call to stream.RecvMsg(), to query for
// the next request, returns an error.
//
// This function is used on the server side of the stream -- the side that will
// receive requests and then send responses in reply. (This does not necessarily
// have to be a gRPC server though, as a stream could allow clients to accept
// and process requests this way). For the client side of the stream, see
// NewStreamAdapter.
func HandleServerStream(stream grpc.Stream, serveFunc interface{}, requestKeyExtractor func(interface{}) interface{}, requestFactory func() interface{}) error {
	t := reflect.TypeOf(serveFunc)
	if t.Kind() != reflect.Func {
		panic("serveFunc must be a function")
	}

	hasContext := false
	okay := false
	switch t.NumIn() {
	case 1:
		if t.In(0).Kind() == reflect.Ptr {
			okay = true
		}
	case 2:
		if typeOfContext.AssignableTo(t.In(0)) && t.In(1).Kind() == reflect.Ptr {
			okay = true
			hasContext = true
		}
	}
	if !okay || t.NumOut() != 1 {
		panic(fmt.Sprintf("serveFunc has bad signature: %v", t))
	}

	funcRv := reflect.ValueOf(serveFunc)
	action := func(ctx context.Context, req interface{}) interface{} {
		// invoke reflectively
		var results []reflect.Value
		if hasContext {
			results = funcRv.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(req)})
		} else {
			results = funcRv.Call([]reflect.Value{reflect.ValueOf(req)})
		}
		return results[0].Interface()
	}

	if requestFactory == nil {
		requestFactory = createFactory(stream)
	}

	w := &workers{
		stream:              stream,
		action:              action,
		requestKeyExtractor: requestKeyExtractor,
	}
	w.ctx, w.cancel = context.WithCancel(stream.Context())
	defer w.await()

	for {
		m := requestFactory()
		if err := stream.RecvMsg(m); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		w.enqueue(m)
	}
}

type workers struct {
	stream              grpc.Stream
	action              func(context.Context, interface{}) interface{}
	requestKeyExtractor func(interface{}) interface{}
	ctx                 context.Context
	cancel              context.CancelFunc
	wg                  sync.WaitGroup

	mu      sync.Mutex
	workers map[interface{}]worker
}

type worker struct {
	input chan<- *workRequest
}

type workRequest struct {
	req  interface{}
	skip int32 // used as atomic boolean; set to 1 to retract request
}

func (w *workers) await() {
	w.wg.Wait()
}

func (w *workers) enqueue(req interface{}) {
	var key interface{}
	if w.requestKeyExtractor != nil {
		key = w.requestKeyExtractor(req)
	}

	for {
		w.mu.Lock()
		wk, ok := w.workers[key]
		if !ok {
			// don't release lock; we need to create new worker below
			break
		}

		// unlock before trying to send worker a request
		w.mu.Unlock()

		wr := &workRequest{req: req}
		select {
		case wk.input <- wr:
			// worker accepted request? double-check that
			// worker didn't asynchronously exit
			w.mu.Lock()
			wk2, active := w.workers[key]
			w.mu.Unlock()
			wkIsActive := active && wk2 == wk
			if !wkIsActive && atomic.CompareAndSwapInt32(&wr.skip, 0, 1) {
				// worker exited and did not process this item,
				// so we need to get a new worker...
				continue
			}
			return
		case <-w.ctx.Done():
			// stream shutting down, bail
			return
		}
	}

	defer w.mu.Unlock()

	input := make(chan *workRequest, 16)
	wk := worker{input: input}
	w.workers[key] = wk

	// TODO: limit parallelism?

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.processRequests(key, input)
	}()
}

func (w *workers) processRequests(key interface{}, input <-chan *workRequest) {
	for {
		var req *workRequest
		select {
		case req = <-input:
			if !w.processOneRequest(req) {
				// stream is in bad state
				return
			}

		case <-w.ctx.Done():
			// operation cancelled
			return

		default:
			// No tasks available; try to quit. Make sure we
			// aren't racing with a goroutine adding something
			// to the input channel.
			w.mu.Lock()
			noExit := false
			select {
			case req = <-input:
				noExit = true
			default:
				delete(w.workers, key)
			}
			w.mu.Unlock()

			if !noExit {
				return
			}

			// otherwise, we found an item we need to process so can't exit yet
			if !w.processOneRequest(req) {
				// stream is in bad state
				return
			}
		}
	}
}

func (w *workers) processOneRequest(req *workRequest) bool {
	if !atomic.CompareAndSwapInt32(&req.skip, 0, -1) {
		// request was already marked, so skip
		return true
	}
	resp := w.action(w.ctx, req)
	if err := w.stream.SendMsg(resp); err != nil {
		w.cancel()
		// stream is in bad state
		return false
	}
	return true
}
