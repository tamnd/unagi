package objects

import "sync"

// asyncio.Queue is a coroutine-level FIFO: put suspends when the queue is full,
// get suspends when it is empty, and join blocks until every put has been marked
// done. Like the other asyncio primitives it lives on the loop goroutine, so its
// fields need no lock of their own; the frame machinery serialises put, get,
// task_done, and join.
type asyncioQueue struct {
	items      []Object
	maxsize    int
	getters    []*asyncFuture
	putters    []*asyncFuture
	unfinished int
	finished   *asyncioEvent
}

func (q *asyncioQueue) TypeName() string { return "Queue" }

// AsyncioNewQueue builds asyncio.Queue(maxsize). A maxsize of zero or less is
// unbounded, matching CPython. The finished event starts set, so join returns at
// once until the first put.
func AsyncioNewQueue(maxsize int) Object {
	return &asyncioQueue{maxsize: maxsize, finished: &asyncioEvent{value: true}}
}

// asyncioQueueMethod dispatches the Queue surface. qsize, empty, and full read
// the state; put_nowait, get_nowait, and task_done act at once; put, get, and
// join hand back the coroutines the caller awaits.
func asyncioQueueMethod(q *asyncioQueue, name string, args []Object) (Object, error) {
	switch name {
	case "qsize":
		if len(args) != 0 {
			return nil, Raise(TypeError, "qsize() takes 1 positional argument but %d were given", len(args)+1)
		}
		return NewInt(int64(len(q.items))), nil
	case "empty":
		if len(args) != 0 {
			return nil, Raise(TypeError, "empty() takes 1 positional argument but %d were given", len(args)+1)
		}
		return NewBool(q.empty()), nil
	case "full":
		if len(args) != 0 {
			return nil, Raise(TypeError, "full() takes 1 positional argument but %d were given", len(args)+1)
		}
		return NewBool(q.full()), nil
	case "put_nowait":
		if len(args) != 1 {
			return nil, Raise(TypeError, "put_nowait() takes exactly one argument (%d given)", len(args))
		}
		return None, q.putNoWait(args[0])
	case "get_nowait":
		if len(args) != 0 {
			return nil, Raise(TypeError, "get_nowait() takes 1 positional argument but %d were given", len(args)+1)
		}
		return q.getNoWait()
	case "put":
		if len(args) != 1 {
			return nil, Raise(TypeError, "put() takes exactly one argument (%d given)", len(args))
		}
		return q.putCoro(args[0]), nil
	case "get":
		if len(args) != 0 {
			return nil, Raise(TypeError, "get() takes 1 positional argument but %d were given", len(args)+1)
		}
		return q.getCoro(), nil
	case "task_done":
		if len(args) != 0 {
			return nil, Raise(TypeError, "task_done() takes 1 positional argument but %d were given", len(args)+1)
		}
		return None, q.taskDone()
	case "join":
		if len(args) != 0 {
			return nil, Raise(TypeError, "join() takes 1 positional argument but %d were given", len(args)+1)
		}
		return q.joinCoro(), nil
	}
	return nil, noAttr(q, name)
}

func (q *asyncioQueue) empty() bool { return len(q.items) == 0 }
func (q *asyncioQueue) full() bool  { return q.maxsize > 0 && len(q.items) >= q.maxsize }

// putNoWait is put_nowait: it stores the item at once, or raises QueueFull when a
// bounded queue is full. Each stored item bumps the unfinished count and clears
// the finished event, so a later join blocks until every item is marked done.
func (q *asyncioQueue) putNoWait(item Object) error {
	if q.full() {
		return raiseAsyncioQueueFull()
	}
	q.items = append(q.items, item)
	q.unfinished++
	q.finished.value = false
	q.wakeupNext(&q.getters)
	return nil
}

// getNoWait is get_nowait: it pops the front item, or raises QueueEmpty when the
// queue is empty. Popping wakes the first blocked putter.
func (q *asyncioQueue) getNoWait() (Object, error) {
	if q.empty() {
		return nil, raiseAsyncioQueueEmpty()
	}
	item := q.items[0]
	q.items = q.items[1:]
	q.wakeupNext(&q.putters)
	return item, nil
}

// putCoro is the coroutine put returns. It parks on a putter future while the
// queue is full, re-checking on each wake since another putter may have taken
// the slot, then stores the item, exactly as CPython's put does.
func (q *asyncioQueue) putCoro(item Object) Object {
	body := func(y Yielder) (Object, error) {
		for q.full() {
			loop := runningLoop.Load()
			if loop == nil {
				return nil, Raise(RuntimeError, "no running event loop")
			}
			putter := &asyncFuture{loop: loop}
			q.putters = append(q.putters, putter)
			if _, err := y.YieldFrom(&futureAwait{f: putter}); err != nil {
				return nil, err
			}
		}
		if err := q.putNoWait(item); err != nil {
			return nil, err
		}
		return None, nil
	}
	return &generatorObject{qual: "Queue.put", body: fromTop(body), ret: None, isCoro: true}
}

// getCoro is the coroutine get returns. It parks on a getter future while the
// queue is empty, re-checking on each wake, then pops the front item.
func (q *asyncioQueue) getCoro() Object {
	body := func(y Yielder) (Object, error) {
		for q.empty() {
			loop := runningLoop.Load()
			if loop == nil {
				return nil, Raise(RuntimeError, "no running event loop")
			}
			getter := &asyncFuture{loop: loop}
			q.getters = append(q.getters, getter)
			if _, err := y.YieldFrom(&futureAwait{f: getter}); err != nil {
				return nil, err
			}
		}
		return q.getNoWait()
	}
	return &generatorObject{qual: "Queue.get", body: fromTop(body), ret: None, isCoro: true}
}

// taskDone is task_done: it marks one delivered item complete and sets the
// finished event when the count reaches zero. Calling it more times than there
// were items is the ValueError CPython raises.
func (q *asyncioQueue) taskDone() error {
	if q.unfinished <= 0 {
		return Raise(ValueError, "task_done() called too many times")
	}
	q.unfinished--
	if q.unfinished == 0 {
		q.finished.set()
	}
	return nil
}

// joinCoro is the coroutine join returns. It waits on the finished event while
// work is outstanding, so it resumes once every put has been marked done.
func (q *asyncioQueue) joinCoro() Object {
	body := func(y Yielder) (Object, error) {
		if q.unfinished > 0 {
			aw, err := Await(q.finished.waitCoro())
			if err != nil {
				return nil, err
			}
			if _, err := y.YieldFrom(aw); err != nil {
				return nil, err
			}
		}
		return None, nil
	}
	return &generatorObject{qual: "Queue.join", body: fromTop(body), ret: None, isCoro: true}
}

// wakeupNext resolves the first pending waiter in the queue, dropping any already
// resolved ahead of it, exactly as CPython's _wakeup_next pops the deque.
func (q *asyncioQueue) wakeupNext(waiters *[]*asyncFuture) {
	for len(*waiters) > 0 {
		w := (*waiters)[0]
		*waiters = (*waiters)[1:]
		if !w.doneP() {
			w.setResult(None)
			return
		}
	}
}

// asyncio.QueueEmpty and asyncio.QueueFull are the Exception subclasses
// get_nowait and put_nowait raise. They live in the asyncio.queues module,
// matching their __module__ in CPython, and are built once on demand.
var (
	asyncioQueueEmptyOnce  sync.Once
	asyncioQueueEmptyClass *classObject
	asyncioQueueFullOnce   sync.Once
	asyncioQueueFullClass  *classObject
)

const asyncioQueuesModule = "asyncio.queues"

// AsyncioQueueEmptyClass returns asyncio.QueueEmpty, raised by get_nowait on an
// empty queue.
func AsyncioQueueEmptyClass() Object {
	asyncioQueueEmptyOnce.Do(func() {
		asyncioQueueEmptyClass = buildAsyncioQueueExc("QueueEmpty")
	})
	return asyncioQueueEmptyClass
}

// AsyncioQueueFullClass returns asyncio.QueueFull, raised by put_nowait on a full
// queue.
func AsyncioQueueFullClass() Object {
	asyncioQueueFullOnce.Do(func() {
		asyncioQueueFullClass = buildAsyncioQueueExc("QueueFull")
	})
	return asyncioQueueFullClass
}

func buildAsyncioQueueExc(name string) *classObject {
	qual := asyncioQueuesModule + "." + name
	c, err := NewClass(name, qual, []Object{ExcClass2("Exception")}, []string{"__module__"}, []Object{NewStr(asyncioQueuesModule)}, nil, nil)
	if err != nil {
		panic("unagi: building " + qual + ": " + err.Error())
	}
	return c.(*classObject)
}

func raiseAsyncioQueueEmpty() error { return instantiateBareExc(AsyncioQueueEmptyClass(), "QueueEmpty") }
func raiseAsyncioQueueFull() error  { return instantiateBareExc(AsyncioQueueFullClass(), "QueueFull") }

// instantiateBareExc builds a no-argument instance of a synthesized exception
// class and returns it as the error to raise, falling back to a RuntimeError if
// the class does not instantiate to an exception.
func instantiateBareExc(cls Object, name string) error {
	inst, err := Instantiate(cls.(*classObject), nil, nil, nil)
	if err != nil {
		return err
	}
	if e, ok := inst.(*Exception); ok {
		return e
	}
	return Raise(RuntimeError, "%s", name)
}
