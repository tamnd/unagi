package objects

import (
	"fmt"
	"sync"
	"time"
)

// simpleQueueObject is queue.SimpleQueue (the _queue.SimpleQueue C type): an
// unbounded FIFO without the task_done and join accounting Queue carries. Because
// it has no maxsize a put never blocks, so only get can wait. The Go form is a
// mutex over an item slice with a FIFO of not-empty waiters, reusing the same park,
// wake, and reclaim helpers Queue uses. CPython builds SimpleQueue in C so it is
// reentrant enough to use from a __del__; the Go form is likewise a plain lock with
// no callbacks under it.
type simpleQueueObject struct {
	mu       sync.Mutex
	items    []Object
	notEmpty []chan struct{}
}

// NewSimpleQueue builds an empty, unbounded SimpleQueue.
func NewSimpleQueue() *simpleQueueObject { return &simpleQueueObject{} }

func (q *simpleQueueObject) TypeName() string { return "SimpleQueue" }

// put appends item and wakes one waiting getter. It never blocks: a SimpleQueue has
// no capacity bound, so CPython accepts block and timeout only for signature
// symmetry with Queue and ignores them.
func (q *simpleQueueObject) put(item Object) {
	q.mu.Lock()
	q.items = append(q.items, item)
	wakeOneWaiter(&q.notEmpty)
	q.mu.Unlock()
}

// get removes and returns the front item, blocking while the queue is empty. It
// reports queue.Empty on a non-blocking miss or a timeout, the same not-empty wait
// Queue.get performs, minus the not-full wake a SimpleQueue has no use for.
func (q *simpleQueueObject) get(block bool, hasTimeout bool, timeout time.Duration) (Object, error) {
	var deadline time.Time
	if hasTimeout {
		deadline = time.Now().Add(timeout)
	}
	q.mu.Lock()
	for len(q.items) == 0 {
		if !block {
			q.mu.Unlock()
			return nil, newQueueEmpty()
		}
		var remaining time.Duration
		if hasTimeout {
			remaining = time.Until(deadline)
			if remaining <= 0 {
				q.mu.Unlock()
				return nil, newQueueEmpty()
			}
		}
		w := make(chan struct{})
		q.notEmpty = append(q.notEmpty, w)
		q.mu.Unlock()

		if !queuePark(w, hasTimeout, remaining) {
			if reclaimWaiter(&q.mu, &q.notEmpty, w) {
				return nil, newQueueEmpty()
			}
		}
		q.mu.Lock()
	}
	item := q.items[0]
	q.items = q.items[1:]
	q.mu.Unlock()
	return item, nil
}

func (q *simpleQueueObject) size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

func (q *simpleQueueObject) isEmpty() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items) == 0
}

func simpleQueueMethod(q *simpleQueueObject, name string, args []Object) (Object, error) {
	switch name {
	case "put", "put_nowait":
		item, err := parseSimplePut(name, args, nil, nil)
		if err != nil {
			return nil, err
		}
		q.put(item)
		return None, nil
	case "get":
		block, hasTimeout, timeout, err := parseQueueGet(args, nil, nil)
		if err != nil {
			return nil, err
		}
		return q.get(block, hasTimeout, timeout)
	case "get_nowait":
		if len(args) != 0 {
			return nil, Raise(TypeError, "get_nowait() takes no arguments (%d given)", len(args))
		}
		return q.get(false, false, 0)
	case "qsize":
		if len(args) != 0 {
			return nil, Raise(TypeError, "qsize() takes no arguments (%d given)", len(args))
		}
		return NewInt(int64(q.size())), nil
	case "empty":
		if len(args) != 0 {
			return nil, Raise(TypeError, "empty() takes no arguments (%d given)", len(args))
		}
		return NewBool(q.isEmpty()), nil
	}
	return nil, noAttr(q, name)
}

func simpleQueueMethodKw(q *simpleQueueObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	switch name {
	case "put", "put_nowait":
		item, err := parseSimplePut(name, pos, kwNames, kwVals)
		if err != nil {
			return nil, err
		}
		q.put(item)
		return None, nil
	case "get":
		block, hasTimeout, timeout, err := parseQueueGet(pos, kwNames, kwVals)
		if err != nil {
			return nil, err
		}
		return q.get(block, hasTimeout, timeout)
	}
	return nil, Raise(TypeError, "%s.%s() takes no keyword arguments", q.TypeName(), name)
}

// parseSimplePut reads the put(item, block=True, timeout=None) signature. A
// SimpleQueue can never be full, so block and timeout are accepted for symmetry
// with Queue and otherwise ignored, exactly as CPython does. put_nowait takes only
// the item.
func parseSimplePut(name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if name == "put_nowait" {
		if len(pos) != 1 || len(kwNames) != 0 {
			return nil, Raise(TypeError, "put_nowait() takes exactly one argument (%d given)", len(pos)+len(kwNames))
		}
		return pos[0], nil
	}
	if len(pos) > 3 {
		return nil, Raise(TypeError, "put() takes at most 3 arguments (%d given)", len(pos))
	}
	params := []string{"item", "block", "timeout"}
	set := map[string]Object{}
	for i, v := range pos {
		set[params[i]] = v
	}
	for i, k := range kwNames {
		if k != "item" && k != "block" && k != "timeout" {
			return nil, Raise(TypeError, "'%s' is an invalid keyword argument for put()", k)
		}
		if _, dup := set[k]; dup {
			return nil, Raise(TypeError, "argument for put() given by name ('%s') and position", k)
		}
		set[k] = kwVals[i]
	}
	item, ok := set["item"]
	if !ok {
		return nil, Raise(TypeError, "put() missing 1 required positional argument: 'item'")
	}
	return item, nil
}

var simpleQueueMethodNames = map[string]bool{
	"put": true, "put_nowait": true, "get": true, "get_nowait": true,
	"qsize": true, "empty": true,
}

func simpleQueueRepr(q *simpleQueueObject) string {
	return fmt.Sprintf("<queue.SimpleQueue at %p>", q)
}
