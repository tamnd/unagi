package objects

import (
	"fmt"
	"sync"
	"time"
)

// queueObject is queue.Queue (spec 2076 doc 10 §2.8): a synchronized FIFO. CPython
// builds it from a mutex and three Conditions over that mutex, one each for the
// not-empty, not-full, and all-tasks-done predicates. The Go form keeps the same
// observable behaviour with a single mutex guarding the item slice and three FIFO
// queues of waiter channels. get parks on notEmpty while the queue is empty and a
// put wakes one; put parks on notFull while a bounded queue is at capacity and a
// get wakes one; join parks on allDone until every put has been matched by a
// task_done. A woken waiter re-checks its predicate under the lock, so a value
// another thread took first sends it back to sleep rather than proceeding on a
// predicate that no longer holds, exactly as CPython's Condition.wait loop does.
type queueObject struct {
	mu         sync.Mutex
	kind       queueKind
	items      []Object
	maxsize    int // zero or negative means unbounded
	unfinished int // puts not yet balanced by task_done
	notEmpty   []chan struct{}
	notFull    []chan struct{}
	allDone    []chan struct{}
}

// queueKind selects the container discipline. CPython's LifoQueue and PriorityQueue
// subclass Queue and override only the _put/_get pair: the FIFO keeps a deque, the
// LIFO a stack, the priority queue a binary heap. Everything else, the maxsize
// bound, the not-empty and not-full waits, and the task_done and join accounting,
// is shared, so the Go form carries the discipline as a discriminator rather than
// three separate types.
type queueKind int

const (
	queueFifo queueKind = iota
	queueLifo
	queuePriority
)

// NewQueue builds a Queue with the given maxsize. A maxsize of zero or below is
// CPython's unbounded queue.
func NewQueue(maxsize int) *queueObject {
	return &queueObject{maxsize: maxsize}
}

// NewLifoQueue builds a LifoQueue: a last-in first-out queue whose get returns the
// most recently put item.
func NewLifoQueue(maxsize int) *queueObject {
	return &queueObject{kind: queueLifo, maxsize: maxsize}
}

// NewPriorityQueue builds a PriorityQueue: get returns the smallest item under
// Python's <, keeping the item slice as a binary heap the way CPython does.
func NewPriorityQueue(maxsize int) *queueObject {
	return &queueObject{kind: queuePriority, maxsize: maxsize}
}

func (q *queueObject) TypeName() string {
	switch q.kind {
	case queueLifo:
		return "LifoQueue"
	case queuePriority:
		return "PriorityQueue"
	default:
		return "Queue"
	}
}

// full reports whether a bounded queue is at capacity. The caller holds the lock.
func (q *queueObject) atCapacity() bool {
	return q.maxsize > 0 && len(q.items) >= q.maxsize
}

// put appends item, blocking while a bounded queue is full. It reports the
// queue.Full exception on a non-blocking miss or a timeout, mirroring the
// not_full Condition CPython waits on.
func (q *queueObject) put(item Object, block bool, hasTimeout bool, timeout time.Duration) error {
	var deadline time.Time
	if hasTimeout {
		deadline = time.Now().Add(timeout)
	}
	q.mu.Lock()
	for q.atCapacity() {
		if !block {
			q.mu.Unlock()
			return newQueueFull()
		}
		var remaining time.Duration
		if hasTimeout {
			remaining = time.Until(deadline)
			if remaining <= 0 {
				q.mu.Unlock()
				return newQueueFull()
			}
		}
		w := make(chan struct{})
		q.notFull = append(q.notFull, w)
		q.mu.Unlock()

		if !queuePark(w, hasTimeout, remaining) {
			// The timer fired. If a get popped this waiter first the timeout lost
			// the race and there is room now, so retry the predicate; otherwise pull
			// the waiter from the queue and report the timeout as Full.
			if q.reclaim(&q.notFull, w) {
				return newQueueFull()
			}
		}
		q.mu.Lock()
	}
	if err := q.enqueue(item); err != nil {
		// A PriorityQueue rejects an item its heap cannot order. CPython leaves the
		// count and the not-empty wake untouched in that case, so a later get sees no
		// spurious task, matching Queue.put where _put runs before the bookkeeping.
		q.mu.Unlock()
		return err
	}
	q.unfinished++
	wakeOneWaiter(&q.notEmpty)
	q.mu.Unlock()
	return nil
}

// get removes and returns the front item, blocking while the queue is empty. It
// reports the queue.Empty exception on a non-blocking miss or a timeout.
func (q *queueObject) get(block bool, hasTimeout bool, timeout time.Duration) (Object, error) {
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
			if q.reclaim(&q.notEmpty, w) {
				return nil, newQueueEmpty()
			}
		}
		q.mu.Lock()
	}
	item, err := q.dequeue()
	if err != nil {
		q.mu.Unlock()
		return nil, err
	}
	wakeOneWaiter(&q.notFull)
	q.mu.Unlock()
	return item, nil
}

// enqueue adds item under the kind's discipline. The caller holds the lock. Only a
// PriorityQueue can report an error, the TypeError raised when the heap compares
// two items that have no ordering.
func (q *queueObject) enqueue(item Object) error {
	if q.kind == queuePriority {
		return q.heapPush(item)
	}
	q.items = append(q.items, item)
	return nil
}

// dequeue removes and returns the next item under the kind's discipline: the front
// for a FIFO, the top of the stack for a LifoQueue, the smallest for a
// PriorityQueue. The caller holds the lock and has checked the queue is non-empty.
func (q *queueObject) dequeue() (Object, error) {
	switch q.kind {
	case queueLifo:
		n := len(q.items) - 1
		item := q.items[n]
		q.items = q.items[:n]
		return item, nil
	case queuePriority:
		return q.heapPop()
	default:
		item := q.items[0]
		q.items = q.items[1:]
		return item, nil
	}
}

// heapPush appends item and sifts it up, keeping the item slice a binary min-heap.
// It mirrors CPython heapq's heappush exactly so a PriorityQueue pops in the same
// order CPython does, comparison for comparison.
func (q *queueObject) heapPush(item Object) error {
	q.items = append(q.items, item)
	return q.siftdown(0, len(q.items)-1)
}

// heapPop removes and returns the smallest item, mirroring CPython heapq's heappop:
// the last leaf moves to the root and sifts down. The last element is popped first,
// so a single-element heap returns it without a comparison.
func (q *queueObject) heapPop() (Object, error) {
	n := len(q.items) - 1
	last := q.items[n]
	q.items = q.items[:n]
	if len(q.items) == 0 {
		return last, nil
	}
	top := q.items[0]
	q.items[0] = last
	if err := q.siftup(0); err != nil {
		return nil, err
	}
	return top, nil
}

// siftdown walks the item at pos up toward startpos while it is smaller than its
// parent, restoring the heap invariant after an append. It is CPython heapq's
// _siftdown, ordering with objLess so the sequence of comparisons matches.
func (q *queueObject) siftdown(startpos, pos int) error {
	newitem := q.items[pos]
	for pos > startpos {
		parentpos := (pos - 1) >> 1
		parent := q.items[parentpos]
		less, err := objLess(newitem, parent)
		if err != nil {
			return err
		}
		if !less {
			break
		}
		q.items[pos] = parent
		pos = parentpos
	}
	q.items[pos] = newitem
	return nil
}

// siftup walks the item at pos down to a leaf along the smaller child, then sifts
// it back to its resting place, CPython heapq's _siftup. The child pick uses "not
// left < right" so ties keep the left child, the same bias CPython has.
func (q *queueObject) siftup(pos int) error {
	endpos := len(q.items)
	startpos := pos
	newitem := q.items[pos]
	childpos := 2*pos + 1
	for childpos < endpos {
		rightpos := childpos + 1
		if rightpos < endpos {
			less, err := objLess(q.items[childpos], q.items[rightpos])
			if err != nil {
				return err
			}
			if !less {
				childpos = rightpos
			}
		}
		q.items[pos] = q.items[childpos]
		pos = childpos
		childpos = 2*pos + 1
	}
	q.items[pos] = newitem
	return q.siftdown(startpos, pos)
}

// objLess reports whether a < b under Python's comparison, the ordering CPython's
// heapq uses. It surfaces the TypeError two unorderable items raise.
func objLess(a, b Object) (bool, error) {
	r, err := Compare(OpLt, a, b)
	if err != nil {
		return false, err
	}
	return Truth(r), nil
}

// taskDone balances one put. It wakes every joiner once the count reaches zero,
// and reports the ValueError CPython raises when it is called more times than
// there were puts.
func (q *queueObject) taskDone() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.unfinished <= 0 {
		return Raise(ValueError, "task_done() called too many times")
	}
	q.unfinished--
	if q.unfinished == 0 {
		wakeAllWaiters(&q.allDone)
	}
	return nil
}

// join blocks until every put has been balanced by a task_done. It carries no
// timeout, the way Queue.join does not.
func (q *queueObject) join() {
	q.mu.Lock()
	for q.unfinished > 0 {
		w := make(chan struct{})
		q.allDone = append(q.allDone, w)
		q.mu.Unlock()
		<-w
		q.mu.Lock()
	}
	q.mu.Unlock()
}

func (q *queueObject) size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

func (q *queueObject) isEmpty() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items) == 0
}

func (q *queueObject) isFull() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.atCapacity()
}

// reclaim pulls a timed-out waiter from one of the queue's waiter lists. It
// returns true when the waiter was still queued, a genuine timeout, and false
// when a wake had already popped it, in which case this waiter now owns the slot.
func (q *queueObject) reclaim(waiters *[]chan struct{}, w chan struct{}) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, c := range *waiters {
		if c == w {
			*waiters = append((*waiters)[:i], (*waiters)[i+1:]...)
			return true
		}
	}
	return false
}

// queuePark blocks on the waiter channel until a wake closes it or the timeout
// elapses, reporting whether it was woken.
func queuePark(w chan struct{}, hasTimeout bool, timeout time.Duration) bool {
	if !hasTimeout {
		<-w
		return true
	}
	tm := time.NewTimer(timeout)
	defer tm.Stop()
	select {
	case <-w:
		return true
	case <-tm.C:
		return false
	}
}

// wakeOneWaiter closes the front waiter of a list so exactly one parked thread
// proceeds. The caller holds the lock.
func wakeOneWaiter(waiters *[]chan struct{}) {
	if len(*waiters) > 0 {
		w := (*waiters)[0]
		*waiters = (*waiters)[1:]
		close(w)
	}
}

// wakeAllWaiters closes every waiter of a list, the broadcast join needs when the
// unfinished count reaches zero. The caller holds the lock.
func wakeAllWaiters(waiters *[]chan struct{}) {
	for _, w := range *waiters {
		close(w)
	}
	*waiters = nil
}

func queueMethod(q *queueObject, name string, args []Object) (Object, error) {
	switch name {
	case "put":
		item, block, hasTimeout, timeout, err := parseQueuePut(args, nil, nil)
		if err != nil {
			return nil, err
		}
		return None, q.put(item, block, hasTimeout, timeout)
	case "put_nowait":
		if len(args) != 1 {
			return nil, Raise(TypeError, "put_nowait() takes exactly one argument (%d given)", len(args))
		}
		return None, q.put(args[0], false, false, 0)
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
	case "task_done":
		if len(args) != 0 {
			return nil, Raise(TypeError, "task_done() takes no arguments (%d given)", len(args))
		}
		return None, q.taskDone()
	case "join":
		if len(args) != 0 {
			return nil, Raise(TypeError, "join() takes no arguments (%d given)", len(args))
		}
		q.join()
		return None, nil
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
	case "full":
		if len(args) != 0 {
			return nil, Raise(TypeError, "full() takes no arguments (%d given)", len(args))
		}
		return NewBool(q.isFull()), nil
	}
	return nil, noAttr(q, name)
}

func queueMethodKw(q *queueObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	switch name {
	case "put":
		item, block, hasTimeout, timeout, err := parseQueuePut(pos, kwNames, kwVals)
		if err != nil {
			return nil, err
		}
		return None, q.put(item, block, hasTimeout, timeout)
	case "get":
		block, hasTimeout, timeout, err := parseQueueGet(pos, kwNames, kwVals)
		if err != nil {
			return nil, err
		}
		return q.get(block, hasTimeout, timeout)
	}
	return nil, Raise(TypeError, "%s.%s() takes no keyword arguments", q.TypeName(), name)
}

// parseQueuePut reads the put(item, block=True, timeout=None) signature.
func parseQueuePut(pos []Object, kwNames []string, kwVals []Object) (Object, bool, bool, time.Duration, error) {
	if len(pos) > 3 {
		return nil, false, false, 0, Raise(TypeError, "put() takes at most 3 arguments (%d given)", len(pos))
	}
	params := []string{"item", "block", "timeout"}
	set := map[string]Object{}
	for i, v := range pos {
		set[params[i]] = v
	}
	for i, k := range kwNames {
		if k != "item" && k != "block" && k != "timeout" {
			return nil, false, false, 0, Raise(TypeError, "'%s' is an invalid keyword argument for put()", k)
		}
		if _, dup := set[k]; dup {
			return nil, false, false, 0, Raise(TypeError, "argument for put() given by name ('%s') and position", k)
		}
		set[k] = kwVals[i]
	}
	item, ok := set["item"]
	if !ok {
		return nil, false, false, 0, Raise(TypeError, "put() missing 1 required positional argument: 'item'")
	}
	block, hasTimeout, timeout, err := parseBlockTimeout(set)
	if err != nil {
		return nil, false, false, 0, err
	}
	return item, block, hasTimeout, timeout, nil
}

// parseQueueGet reads the get(block=True, timeout=None) signature.
func parseQueueGet(pos []Object, kwNames []string, kwVals []Object) (bool, bool, time.Duration, error) {
	if len(pos) > 2 {
		return false, false, 0, Raise(TypeError, "get() takes at most 2 arguments (%d given)", len(pos))
	}
	params := []string{"block", "timeout"}
	set := map[string]Object{}
	for i, v := range pos {
		set[params[i]] = v
	}
	for i, k := range kwNames {
		if k != "block" && k != "timeout" {
			return false, false, 0, Raise(TypeError, "'%s' is an invalid keyword argument for get()", k)
		}
		if _, dup := set[k]; dup {
			return false, false, 0, Raise(TypeError, "argument for get() given by name ('%s') and position", k)
		}
		set[k] = kwVals[i]
	}
	return parseBlockTimeout(set)
}

// parseBlockTimeout reads the shared (block, timeout) pair get and put take.
// CPython only validates the timeout when block is true: a non-blocking call
// ignores the timeout entirely, and a blocking call rejects a negative one with
// the "'timeout' must be a non-negative number" ValueError. A None timeout blocks
// forever.
func parseBlockTimeout(set map[string]Object) (bool, bool, time.Duration, error) {
	block := true
	if b, ok := set["block"]; ok {
		block = Truth(b)
	}
	if !block {
		return false, false, 0, nil
	}
	tv, ok := set["timeout"]
	if !ok || tv == None {
		return true, false, 0, nil
	}
	f, ok := AsFloat(tv)
	if !ok {
		return false, false, 0, Raise(TypeError, "'%s' object cannot be interpreted as an integer or float", tv.TypeName())
	}
	if f < 0 {
		return false, false, 0, Raise(ValueError, "'timeout' must be a non-negative number")
	}
	return true, true, time.Duration(f * float64(time.Second)), nil
}

var queueMethodNames = map[string]bool{
	"put": true, "put_nowait": true, "get": true, "get_nowait": true,
	"task_done": true, "join": true, "qsize": true, "empty": true, "full": true,
}

// queueProperties are the read-only value attributes Queue exposes.
var queueProperties = map[string]bool{"maxsize": true}

// queueProperty reads one of Queue's value attributes. maxsize reports zero for
// an unbounded queue, the constructor default.
func queueProperty(q *queueObject, name string) Object {
	if name == "maxsize" {
		if q.maxsize < 0 {
			return NewInt(0)
		}
		return NewInt(int64(q.maxsize))
	}
	return nil
}

func queueRepr(q *queueObject) string {
	q.mu.Lock()
	n := len(q.items)
	q.mu.Unlock()
	return fmt.Sprintf("<queue.%s at %p maxsize=%d _qsize=%d>", q.TypeName(), q, q.maxsize, n)
}

// queue.Empty and queue.Full are the exceptions the non-blocking and timed calls
// raise. CPython puts Empty in the _queue C extension and Full in queue.py, so
// their qualified names differ, but both are plain Exception subclasses carrying
// no message. Each is built once on first use, after excclass.go's init has
// populated the Exception base a program catches them through.
var (
	queueEmptyOnce  sync.Once
	queueEmptyClass *classObject
	queueFullOnce   sync.Once
	queueFullClass  *classObject
)

// QueueEmptyClass returns the queue.Empty class object, spelled _queue.Empty the
// way CPython reports it.
func QueueEmptyClass() Object {
	queueEmptyOnce.Do(func() {
		queueEmptyClass = buildQueueExc("Empty", "_queue.Empty", "_queue")
	})
	return queueEmptyClass
}

// QueueFullClass returns the queue.Full class object.
func QueueFullClass() Object {
	queueFullOnce.Do(func() {
		queueFullClass = buildQueueExc("Full", "queue.Full", "queue")
	})
	return queueFullClass
}

// buildQueueExc constructs one of the queue exception classes against the
// Exception base, recording its module so __module__ and __qualname__ read the
// way CPython reports them.
func buildQueueExc(name, qual, module string) *classObject {
	base, ok := ExcClass("Exception")
	if !ok {
		panic("unagi: Exception class unavailable for " + qual)
	}
	c, err := NewClass(name, qual, []Object{base}, []string{"__module__"}, []Object{NewStr(module)}, nil, nil)
	if err != nil {
		panic("unagi: building " + qual + ": " + err.Error())
	}
	return c.(*classObject)
}

// newQueueEmpty builds a queue.Empty instance ready to raise, carrying no message
// the way CPython raises it.
func newQueueEmpty() error { return instantiateQueueExc(QueueEmptyClass(), ValueError) }

// newQueueFull builds a queue.Full instance ready to raise.
func newQueueFull() error { return instantiateQueueExc(QueueFullClass(), ValueError) }

func instantiateQueueExc(class Object, fallback string) error {
	inst, err := Instantiate(class.(*classObject), nil, nil, nil)
	if err != nil {
		return err
	}
	if e, ok := inst.(error); ok {
		return e
	}
	return Raise(fallback, "%s", class.(*classObject).name)
}
