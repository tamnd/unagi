package objects

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// executorObject is concurrent.futures.ThreadPoolExecutor (spec 2076 doc 10
// §2.9): a pool of worker goroutines draining a shared work queue, each item a
// callable whose result lands in the Future submit handed back. CPython backs it
// with a SimpleQueue and a set of daemon-free worker threads spun up lazily as
// work arrives; the Go form keeps the same shape over goroutines, a slice queue
// under a mutex, and a Cond standing in for the queue's blocking get.
//
// Workers are non-daemon so a program that submits work and reads the results
// runs to completion, and the process-exit drain in ShutdownExecutors joins them
// the way concurrent.futures._python_exit does for a pool a program forgot to
// shut down.
//
// This covers submit, map, shutdown, the context-manager protocol, and the
// initializer/initargs and BrokenThreadPool machinery. The stderr traceback
// CPython prints when an initializer raises is a later slice; the broken pool
// still raises BrokenThreadPool from submit and from every pending result.
type executorObject struct {
	mu         sync.Mutex
	cond       *sync.Cond // over mu, signalling a new work item or shutdown
	maxWorkers int
	namePrefix string // worker thread name stem, "prefix_N"
	queue      []*workItem
	numWorkers int  // worker goroutines spawned so far, up to maxWorkers
	idle       int  // workers currently parked on cond, waiting for work
	shutdown   bool // set by shutdown(): no new work is accepted
	workers    sync.WaitGroup
	// initializer runs once at the start of each worker, with initArgs splatted in.
	// One that raises breaks the pool: broken is set, every queued and later
	// submitted future fails with BrokenThreadPool.
	initializer Object
	initArgs    Object
	broken      bool
}

// workItem is one submitted call: the Future to resolve and the callable with
// its arguments, mirroring CPython's _WorkItem.
type workItem struct {
	future  *futureObject
	fn      Object
	args    []Object
	kwNames []string
	kwVals  []Object
}

// poolCounter backs the default "ThreadPoolExecutor-N" thread name prefix.
// CPython advances a shared counter only for an executor built without an
// explicit thread_name_prefix, so an executor that names its own threads never
// consumes a number and the default sequence matches run to run.
var poolCounter atomic.Int64

// NewExecutor builds a ThreadPoolExecutor with the given worker cap and thread
// name prefix. An empty prefix takes the default "ThreadPoolExecutor-N" stem,
// consuming the next pool number, so a worker's threading.current_thread().name
// reads the way CPython spells it.
func NewExecutor(maxWorkers int, namePrefix string) *executorObject {
	if namePrefix == "" {
		namePrefix = fmt.Sprintf("ThreadPoolExecutor-%d", poolCounter.Add(1)-1)
	}
	e := &executorObject{maxWorkers: maxWorkers, namePrefix: namePrefix}
	e.cond = sync.NewCond(&e.mu)
	registerExecutor(e)
	return e
}

// SetInitializer records the callable each worker runs before it drains any
// work, with args splatted in as the positional arguments. args is the
// initargs tuple; it is unpacked lazily at worker start, so a bad initargs
// surfaces as the initializer call failing, breaking the pool, the way CPython
// defers it to the worker.
func (e *executorObject) SetInitializer(fn, args Object) {
	e.initializer = fn
	e.initArgs = args
}

func (e *executorObject) TypeName() string { return "ThreadPoolExecutor" }

// submit enqueues fn(*args, **kw) and returns the pending Future for it. A pool
// already shut down raises the RuntimeError CPython raises from submit. A fresh
// item wakes a parked worker or, when none is idle and the pool is below its
// cap, spawns one, the lazy growth _adjust_thread_count does.
func (e *executorObject) submit(fn Object, args []Object, kwNames []string, kwVals []Object) (Object, error) {
	f := NewFuture()
	w := &workItem{future: f, fn: fn, args: args, kwNames: kwNames, kwVals: kwVals}
	e.mu.Lock()
	if e.broken {
		e.mu.Unlock()
		return nil, raiseBrokenThreadPool()
	}
	if e.shutdown {
		e.mu.Unlock()
		return nil, Raise(RuntimeError, "cannot schedule new futures after shutdown")
	}
	e.queue = append(e.queue, w)
	if e.idle == 0 && e.numWorkers < e.maxWorkers {
		e.spawnWorker()
	} else {
		e.cond.Signal()
	}
	e.mu.Unlock()
	return f, nil
}

// spawnWorker starts one worker goroutine under a fresh non-daemon Thread. The
// caller holds e.mu. The worker name is "prefix_N" with N the zero-based index
// of the worker in the pool, the name CPython gives its pool threads. The worker
// is counted against the pool's WaitGroup so shutdown(wait=True) can join it.
func (e *executorObject) spawnWorker() {
	name := fmt.Sprintf("%s_%d", e.namePrefix, e.numWorkers)
	e.numWorkers++
	th := NewThread(name, false)
	// A pool worker is a threading.Thread in CPython, so current_thread() inside a
	// task reads the worker's name. Give the state a started Thread wrapper carrying
	// that name, the object current_thread hands back on this goroutine.
	th.SetWrapper(&threadObject{clsName: "Thread", name: name, started: true, state: th})
	e.workers.Add(1)
	SpawnFunc(th, func() {
		defer e.workers.Done()
		e.workerLoop(th)
	})
}

// workerLoop is the body each worker goroutine runs: pull the next item and run
// it, parking on the cond when the queue is empty. A worker exits once the pool
// is shut down and the queue has drained, so shutdown(wait=True) joins a pool
// with no work left, while shutdown drains any remaining items first.
func (e *executorObject) workerLoop(th *Thread) {
	if e.initializer != nil {
		if err := e.runInitializer(th); err != nil {
			e.breakPool(th)
			return
		}
	}
	for {
		e.mu.Lock()
		for len(e.queue) == 0 && !e.shutdown {
			e.idle++
			e.cond.Wait()
			e.idle--
		}
		if len(e.queue) == 0 {
			e.mu.Unlock()
			return
		}
		w := e.queue[0]
		e.queue = e.queue[1:]
		e.mu.Unlock()
		e.runItem(th, w)
	}
}

// runItem runs one work item under the worker's Thread and resolves its Future,
// the body CPython's _WorkItem.run runs: skip a cancelled item, otherwise call
// the target and record the result or the exception it raised, then fire the
// done callbacks. A callback runs on the worker thread, matching CPython.
func (e *executorObject) runItem(th *Thread, w *workItem) {
	ok, err := w.future.setRunningOrNotifyCancel()
	if err != nil || !ok {
		// A cancelled item is skipped; the unexpected-state error cannot arise
		// here since only this worker transitions the future out of pending.
		return
	}
	var res Object
	if len(w.kwNames) == 0 {
		res, err = CallT(th, w.fn, w.args)
	} else {
		res, err = CallKwT(th, w.fn, w.args, w.kwNames, w.kwVals)
	}
	var cbs []Object
	if err != nil {
		cbs, _ = w.future.setException(excFromError(err))
	} else {
		cbs, _ = w.future.setResult(res)
	}
	invokeFutureCallbacks(th, w.future, cbs)
}

// runInitializer runs the pool's initializer once on the worker thread, with
// the initargs tuple splatted in as positional arguments. An empty or nil
// initargs runs it with no arguments.
func (e *executorObject) runInitializer(th *Thread) error {
	var args []Object
	if e.initArgs != nil && e.initArgs != None {
		unpacked, err := unpackSequence(e.initArgs)
		if err != nil {
			return err
		}
		args = unpacked
	}
	_, err := CallT(th, e.initializer, args)
	return err
}

// breakPool marks the pool unusable after an initializer failed and fails every
// queued future with BrokenThreadPool, the state CPython's _initializer_failed
// leaves the pool in. New submits raise BrokenThreadPool too, checked in submit.
func (e *executorObject) breakPool(th *Thread) {
	e.mu.Lock()
	e.broken = true
	queued := e.queue
	e.queue = nil
	e.cond.Broadcast()
	e.mu.Unlock()
	for _, w := range queued {
		if cbs, err := w.future.setException(brokenThreadPoolExc()); err == nil {
			invokeFutureCallbacks(th, w.future, cbs)
		}
	}
}

// unpackSequence flattens any iterable into a slice, used to splat an initargs
// tuple into the initializer call.
func unpackSequence(o Object) ([]Object, error) {
	it, err := Iter(o)
	if err != nil {
		return nil, err
	}
	var out []Object
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return out, nil
		}
		out = append(out, v)
	}
}

// BrokenExecutor and BrokenThreadPool are built once on first use, after the
// exception hierarchy in excclass.go's init has populated RuntimeError.
// BrokenExecutor is concurrent.futures._base.BrokenExecutor, a RuntimeError
// subclass; BrokenThreadPool is concurrent.futures.thread.BrokenThreadPool, its
// subclass, the class CPython raises once a worker initializer fails.
var (
	brokenExecutorOnce    sync.Once
	brokenExecutorClass   *classObject
	brokenThreadPoolOnce  sync.Once
	brokenThreadPoolClass *classObject
)

// BrokenExecutorClass returns concurrent.futures._base.BrokenExecutor.
func BrokenExecutorClass() Object {
	brokenExecutorOnce.Do(func() {
		base, ok := ExcClass("RuntimeError")
		if !ok {
			panic("unagi: RuntimeError class unavailable for BrokenExecutor")
		}
		c, err := NewClass("BrokenExecutor", "concurrent.futures._base.BrokenExecutor",
			[]Object{base}, []string{"__module__"}, []Object{NewStr("concurrent.futures")}, nil, nil)
		if err != nil {
			panic("unagi: building BrokenExecutor: " + err.Error())
		}
		brokenExecutorClass = c.(*classObject)
	})
	return brokenExecutorClass
}

// BrokenThreadPoolClass returns concurrent.futures.thread.BrokenThreadPool.
func BrokenThreadPoolClass() Object {
	brokenThreadPoolOnce.Do(func() {
		c, err := NewClass("BrokenThreadPool", "concurrent.futures.thread.BrokenThreadPool",
			[]Object{BrokenExecutorClass()}, []string{"__module__"}, []Object{NewStr("concurrent.futures.thread")}, nil, nil)
		if err != nil {
			panic("unagi: building BrokenThreadPool: " + err.Error())
		}
		brokenThreadPoolClass = c.(*classObject)
	})
	return brokenThreadPoolClass
}

// brokenThreadPoolMessage is the text CPython's _initializer_failed sets on the
// BrokenThreadPool it raises from submit and stores on every pending future.
const brokenThreadPoolMessage = "A thread initializer failed, the thread pool is not usable anymore"

// brokenThreadPoolExc builds a BrokenThreadPool instance carrying the standard
// message, the object a broken pool stores on its pending futures.
func brokenThreadPoolExc() Object {
	inst, err := Instantiate(BrokenThreadPoolClass().(*classObject), []Object{NewStr(brokenThreadPoolMessage)}, nil, nil)
	if err != nil {
		return Raise(RuntimeError, "%s", brokenThreadPoolMessage)
	}
	return inst
}

// raiseBrokenThreadPool is the error submit returns once the pool is broken.
func raiseBrokenThreadPool() error {
	if e, ok := brokenThreadPoolExc().(error); ok {
		return e
	}
	return Raise(RuntimeError, "%s", brokenThreadPoolMessage)
}

// excFromError turns the error a call returned into the exception object the
// Future stores. A raised Python exception already is an Object; a bare Go error
// is wrapped in a RuntimeError so the future still carries something raisable.
func excFromError(err error) Object {
	if o, ok := err.(Object); ok {
		return o
	}
	return Raise(RuntimeError, "%s", err.Error())
}

// mapCall submits fn across the rows zipped from the iterables and returns the
// ordered result iterator ThreadPoolExecutor.map hands back. Every task is
// submitted up front, so a side effect in fn runs as the pool schedules it, not
// as the caller consumes the iterator. A total timeout, when given, bounds the
// whole consumption the way CPython's end_time deadline does.
func (e *executorObject) mapCall(fn Object, iterables []Object, hasDeadline bool, deadline time.Time) (Object, error) {
	its := make([]Iterator, len(iterables))
	for i, a := range iterables {
		it, err := Iter(a)
		if err != nil {
			return nil, err
		}
		its[i] = it
	}
	var fs []*futureObject
	for {
		row := make([]Object, len(its))
		stop := false
		for i, it := range its {
			v, ok, err := it.Next()
			if err != nil {
				return nil, err
			}
			if !ok {
				stop = true
				break
			}
			row[i] = v
		}
		if stop {
			break
		}
		f, err := e.submit(fn, row, nil, nil)
		if err != nil {
			return nil, err
		}
		fs = append(fs, f.(*futureObject))
	}
	return &mapResultIter{fs: fs, hasDeadline: hasDeadline, deadline: deadline}, nil
}

// mapResultIter yields map's results in submission order, blocking on each
// future in turn and raising the first exception a task carried. CPython's
// generator cancels the futures it never reaches when iteration ends early or a
// task raises, so this cancels the remaining futures on an error.
type mapResultIter struct {
	fs          []*futureObject
	i           int
	hasDeadline bool
	deadline    time.Time
}

func (m *mapResultIter) TypeName() string           { return "generator" }
func (m *mapResultIter) Iterate() (Iterator, error) { return m, nil }

func (m *mapResultIter) Next() (Object, bool, error) {
	if m.i >= len(m.fs) {
		return nil, false, nil
	}
	f := m.fs[m.i]
	m.i++
	var v Object
	var err error
	if m.hasDeadline {
		v, err = f.result(true, true, time.Until(m.deadline))
	} else {
		v, err = f.result(true, false, 0)
	}
	if err != nil {
		for _, rf := range m.fs[m.i:] {
			rf.cancel()
		}
		return nil, false, err
	}
	return v, true, nil
}

// doShutdown stops the pool and, when asked, joins its workers. It sets the
// shutdown flag so submit refuses new work, optionally cancels the still-queued
// futures, wakes every parked worker so they see the flag, and on wait blocks
// until the workers have drained and returned. It is idempotent: a second call,
// including the process-exit drain, finds the flag already set and simply waits.
func (e *executorObject) doShutdown(t *Thread, wait, cancelFutures bool) {
	e.mu.Lock()
	e.shutdown = true
	var futs []*futureObject
	var fired [][]Object
	if cancelFutures {
		for _, w := range e.queue {
			if _, cbs := w.future.cancel(); len(cbs) > 0 {
				futs = append(futs, w.future)
				fired = append(fired, cbs)
			}
		}
		e.queue = nil
	}
	e.cond.Broadcast()
	e.mu.Unlock()
	for i, cbs := range fired {
		invokeFutureCallbacks(t, futs[i], cbs)
	}
	if wait {
		e.workers.Wait()
	}
}

// executorMethodT dispatches the positional executor methods. submit and map
// take the target plus its arguments; shutdown takes an optional positional
// wait; __enter__ and __exit__ drive the with statement.
func executorMethodT(t *Thread, e *executorObject, name string, args []Object) (Object, error) {
	switch name {
	case "submit":
		if len(args) == 0 {
			return nil, Raise(TypeError, "ThreadPoolExecutor.submit() missing 1 required positional argument: 'fn'")
		}
		return e.submit(args[0], args[1:], nil, nil)
	case "map":
		if len(args) == 0 {
			return nil, Raise(TypeError, "Executor.map() missing 1 required positional argument: 'fn'")
		}
		return e.mapCall(args[0], args[1:], false, time.Time{})
	case "shutdown":
		wait, cancelFutures, err := parseShutdownArgs(args, nil, nil)
		if err != nil {
			return nil, err
		}
		e.doShutdown(t, wait, cancelFutures)
		return None, nil
	case "__enter__":
		if len(args) != 0 {
			return nil, Raise(TypeError, "__enter__() takes 1 positional argument but %d were given", len(args)+1)
		}
		return e, nil
	case "__exit__":
		// The with statement passes the three exception slots; a pool always
		// shuts down on the way out and never suppresses the exception.
		e.doShutdown(t, true, false)
		return NewBool(false), nil
	}
	return nil, noAttr(e, name)
}

// executorMethodKwT dispatches the executor methods that take keyword arguments:
// submit passes every keyword on to the target, map reads its own timeout,
// chunksize, and buffersize, and shutdown reads wait and cancel_futures.
func executorMethodKwT(t *Thread, e *executorObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	switch name {
	case "submit":
		if len(pos) == 0 {
			return nil, Raise(TypeError, "ThreadPoolExecutor.submit() missing 1 required positional argument: 'fn'")
		}
		return e.submit(pos[0], pos[1:], kwNames, kwVals)
	case "map":
		return e.mapKw(pos, kwNames, kwVals)
	case "shutdown":
		wait, cancelFutures, err := parseShutdownArgs(pos, kwNames, kwVals)
		if err != nil {
			return nil, err
		}
		e.doShutdown(t, wait, cancelFutures)
		return None, nil
	}
	return executorMethodT(t, e, name, pos)
}

// mapKw reads map's keyword surface: timeout (None or a number of seconds),
// chunksize (ignored by a thread pool), and buffersize (None or a positive int
// here only for its validation, since this slice submits every task up front).
func (e *executorObject) mapKw(pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if len(pos) == 0 {
		return nil, Raise(TypeError, "Executor.map() missing 1 required positional argument: 'fn'")
	}
	hasDeadline := false
	var deadline time.Time
	for i, k := range kwNames {
		switch k {
		case "timeout":
			if kwVals[i] != None {
				secs, ok := AsFloat(kwVals[i])
				if !ok {
					return nil, Raise(TypeError, "'%s' object cannot be interpreted as a float", kwVals[i].TypeName())
				}
				hasDeadline = true
				deadline = time.Now().Add(max(time.Duration(secs*float64(time.Second)), 0))
			}
		case "chunksize":
			// ThreadPoolExecutor ignores chunksize; it only chunks process pools.
		case "buffersize":
			if kwVals[i] != None {
				n, ok := AsInt(kwVals[i])
				if !ok {
					return nil, Raise(TypeError, "buffersize must be an integer or None")
				}
				if n < 1 {
					return nil, Raise(ValueError, "buffersize must be None or > 0")
				}
			}
		default:
			return nil, Raise(TypeError, "Executor.map() got an unexpected keyword argument '%s'", k)
		}
	}
	return e.mapCall(pos[0], pos[1:], hasDeadline, deadline)
}

// parseShutdownArgs reads shutdown(wait=True, *, cancel_futures=False). wait may
// be positional or keyword; cancel_futures is keyword only. An unknown keyword
// is the TypeError CPython raises against the ThreadPoolExecutor.shutdown name.
func parseShutdownArgs(pos []Object, kwNames []string, kwVals []Object) (bool, bool, error) {
	wait, cancelFutures := true, false
	waitSet := false
	if len(pos) > 1 {
		return false, false, Raise(TypeError, "shutdown() takes from 1 to 2 positional arguments but %d were given", len(pos)+1)
	}
	if len(pos) == 1 {
		wait, waitSet = Truth(pos[0]), true
	}
	for i, k := range kwNames {
		switch k {
		case "wait":
			if waitSet {
				return false, false, Raise(TypeError, "shutdown() got multiple values for argument 'wait'")
			}
			wait = Truth(kwVals[i])
		case "cancel_futures":
			cancelFutures = Truth(kwVals[i])
		default:
			return false, false, Raise(TypeError, "ThreadPoolExecutor.shutdown() got an unexpected keyword argument '%s'", k)
		}
	}
	return wait, cancelFutures, nil
}

var executorMethodNames = map[string]bool{
	"submit": true, "map": true, "shutdown": true, "__enter__": true, "__exit__": true,
}

func executorRepr(e *executorObject) string {
	return fmt.Sprintf("<ThreadPoolExecutor at %p>", e)
}

// The live-executor registry backs the process-exit drain. CPython keeps a
// weakref map of executors to their queues and, at interpreter shutdown, tells
// each to stop and joins its threads; the Go form keeps a plain slice since the
// process is exiting and the executors need not be collectable first.
var (
	executorsMu   sync.Mutex
	liveExecutors []*executorObject
)

func registerExecutor(e *executorObject) {
	executorsMu.Lock()
	liveExecutors = append(liveExecutors, e)
	executorsMu.Unlock()
}

// ShutdownExecutors stops every live executor and joins its workers, the drain
// concurrent.futures._python_exit runs at interpreter shutdown. Emitted main
// calls it before it waits on the non-daemon threads, so a pool a program never
// shut down still runs its queued work to completion and lets its worker
// goroutines exit, rather than blocking the process forever.
func ShutdownExecutors() {
	executorsMu.Lock()
	es := append([]*executorObject(nil), liveExecutors...)
	executorsMu.Unlock()
	for _, e := range es {
		e.doShutdown(mainThread, true, false)
	}
}
