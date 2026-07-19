package objects

import (
	"reflect"
	"sync"
	"time"
)

// wait and as_completed are the concurrent.futures module functions that watch a
// group of futures rather than one (spec 2076 doc 10 §2.9). CPython drives them
// with a shared _Waiter installed on every future's Condition; the Go form parks
// on the futures' done channels instead, re-deriving the done set on each wake.
// The observable contract is the same: wait returns the done and not-done sets
// under the chosen return condition, and as_completed yields the futures in the
// order they finish.

// The three return_when constants wait accepts. They are plain strings in
// CPython too, compared by value, so the module binds these same words.
const (
	firstCompleted = "FIRST_COMPLETED"
	firstException = "FIRST_EXCEPTION"
	allCompleted   = "ALL_COMPLETED"
)

// doneAndNotDoneOnce builds the DoneAndNotDoneFutures namedtuple class lazily,
// after namedtuple's own machinery is ready. wait returns an instance of it, a
// two-field tuple of the done and not-done sets.
var (
	doneAndNotDoneOnce  sync.Once
	doneAndNotDoneClass Object
)

func doneAndNotDoneType() Object {
	doneAndNotDoneOnce.Do(func() {
		c, err := NewNamedTupleType("DoneAndNotDoneFutures", []string{"done", "not_done"}, nil)
		if err != nil {
			panic("unagi: building DoneAndNotDoneFutures: " + err.Error())
		}
		doneAndNotDoneClass = c
	})
	return doneAndNotDoneClass
}

// futuresOf turns the fs argument into the futures wait and as_completed watch,
// deduplicating on identity the way CPython's `fs = set(fs)` does while keeping
// the first-seen order for a deterministic initial scan. A non-future element
// fails the way CPython does when it reaches for the missing _condition.
func futuresOf(fs Object) ([]*futureObject, error) {
	it, err := Iter(fs)
	if err != nil {
		return nil, err
	}
	var out []*futureObject
	seen := map[*futureObject]bool{}
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		f, ok := v.(*futureObject)
		if !ok {
			return nil, Raise(AttributeError, "'%s' object has no attribute '_condition'", v.TypeName())
		}
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out, nil
}

// doneNow scans the futures and returns the ones wait counts as done, keeping
// input order. The second result reports whether every future is done.
func doneNow(fs []*futureObject) ([]*futureObject, bool) {
	var done []*futureObject
	for _, f := range fs {
		if f.completedForWait() {
			done = append(done, f)
		}
	}
	return done, len(done) == len(fs)
}

// notDoneOf returns the futures not in the done slice, keeping input order. The
// done slice is a subset drawn from fs in the same order, so a single pass with
// a membership set splits the two.
func notDoneOf(fs, done []*futureObject) []*futureObject {
	inDone := make(map[*futureObject]bool, len(done))
	for _, f := range done {
		inDone[f] = true
	}
	var notDone []*futureObject
	for _, f := range fs {
		if !inDone[f] {
			notDone = append(notDone, f)
		}
	}
	return notDone
}

// waitResult packages the done and not-done futures into the DoneAndNotDoneFutures
// namedtuple of two sets wait returns.
func waitResult(fs, done []*futureObject) (Object, error) {
	notDone := notDoneOf(fs, done)
	doneSet, err := futureSet(done)
	if err != nil {
		return nil, err
	}
	notDoneSet, err := futureSet(notDone)
	if err != nil {
		return nil, err
	}
	return Call(doneAndNotDoneType(), []Object{doneSet, notDoneSet})
}

// futureSet builds a set from a slice of futures.
func futureSet(fs []*futureObject) (Object, error) {
	elts := make([]Object, len(fs))
	for i, f := range fs {
		elts[i] = f
	}
	return NewSet(elts)
}

// firstExceptionMet reports whether the FIRST_EXCEPTION early return fires: any
// already-done future finished with an exception. A cancelled future carries
// none, so only a finished-with-exception one satisfies it.
func firstExceptionMet(done []*futureObject) bool {
	for _, f := range done {
		if f.hasWaitException() {
			return true
		}
	}
	return false
}

// Wait implements concurrent.futures.wait(fs, timeout, return_when). It returns
// the DoneAndNotDoneFutures namedtuple of the done and not-done sets once the
// return condition is met or the timeout elapses. return_when is validated only
// when at least one future is still pending, matching CPython, which reaches the
// check inside _create_and_install_waiters only after the all-done short circuit;
// so wait over an all-done group never rejects an invalid condition. The value
// arrives as an object because CPython compares it by equality to the constant
// strings and reports it with %r on a miss, which prints a non-string as itself.
func Wait(fs Object, hasTimeout bool, timeout time.Duration, returnWhen Object) (Object, error) {
	futs, err := futuresOf(fs)
	if err != nil {
		return nil, err
	}
	rw, isStr := AsStr(returnWhen)
	valid := isStr && (rw == firstCompleted || rw == firstException || rw == allCompleted)

	done, allDone := doneNow(futs)
	switch {
	case rw == firstCompleted && len(done) > 0:
		return waitResult(futs, done)
	case rw == firstException && len(done) > 0 && firstExceptionMet(done):
		return waitResult(futs, done)
	}
	if allDone {
		return waitResult(futs, done)
	}
	if !valid {
		return nil, Raise(ValueError, "Invalid return condition: %s", Repr(returnWhen))
	}

	var deadline time.Time
	if hasTimeout {
		deadline = time.Now().Add(timeout)
	}
	for {
		done, allDone = doneNow(futs)
		if waitConditionMet(rw, done, allDone) {
			return waitResult(futs, done)
		}
		if hasTimeout && !time.Now().Before(deadline) {
			return waitResult(futs, done)
		}
		if !parkForCompletion(futs, done, hasTimeout, deadline) {
			// The park timed out; report whatever is done at the deadline.
			done, _ = doneNow(futs)
			return waitResult(futs, done)
		}
	}
}

// waitConditionMet reports whether the current done set satisfies return_when.
// FIRST_COMPLETED needs one done future, FIRST_EXCEPTION needs one that raised
// or every future done, and ALL_COMPLETED needs every future done.
func waitConditionMet(returnWhen string, done []*futureObject, allDone bool) bool {
	switch returnWhen {
	case firstCompleted:
		return len(done) > 0
	case firstException:
		return allDone || firstExceptionMet(done)
	default:
		return allDone
	}
}

// parkForCompletion blocks until another future completes or the deadline
// passes, reporting whether a completion woke it. It parks on the done channels
// of the not-yet-done futures; a future whose channel is already closed but that
// does not count as done, a plain cancelled one the executor may still notify,
// is polled instead of parked so the select never spins on it. It returns true
// on any wake so the caller re-derives the done set, and false only when the
// deadline case fired.
func parkForCompletion(fs, done []*futureObject, hasTimeout bool, deadline time.Time) bool {
	inDone := make(map[*futureObject]bool, len(done))
	for _, f := range done {
		inDone[f] = true
	}
	var cases []reflect.SelectCase
	orphaned := false
	for _, f := range fs {
		if inDone[f] {
			continue
		}
		ch := f.waitChannel()
		select {
		case <-ch:
			// Closed but not counted as done: a plain cancelled future waiting on
			// the executor's notify. Parking on it would spin, so mark that a poll
			// is needed to catch the later transition and skip the channel.
			orphaned = true
		default:
			cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)})
		}
	}
	if hasTimeout {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false
		}
		cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(time.After(remaining))})
	}
	if orphaned {
		// Re-check the orphaned futures soon; their notify transition sends no
		// signal, so a short poll is the only way to observe it.
		cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(time.After(time.Millisecond))})
	}
	if len(cases) == 0 {
		// No open channel, no timeout, no orphan should be unreachable: the caller
		// only parks with a future still not done, which is either open or orphaned.
		// Poll rather than block, so a mistaken read re-checks instead of hanging.
		time.Sleep(time.Millisecond)
		return true
	}
	chosen, _, _ := reflect.Select(cases)
	// The timeout case, when present, is the last real case unless a poll case
	// follows it. Only a fired timeout returns false; a poll wake re-checks.
	if hasTimeout && chosen == len(cases)-1-boolToInt(orphaned) {
		return false
	}
	return true
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// asCompletedIter is the generator concurrent.futures.as_completed returns: it
// yields the watched futures in the order they finish, deduplicated, and raises
// TimeoutError when the deadline passes with futures still pending. CPython
// yields the already-finished ones first, then blocks for the rest; this keeps
// the same shape over the futures' done channels.
type asCompletedIter struct {
	all         []*futureObject
	yielded     map[*futureObject]bool
	buffer      []*futureObject // done, dedup, awaiting a Next call
	total       int
	hasDeadline bool
	deadline    time.Time
}

// AsCompleted implements concurrent.futures.as_completed(fs, timeout). It returns
// an iterator over the futures in completion order. The timeout bounds the whole
// consumption from this call, the deadline CPython pins with end_time.
func AsCompleted(fs Object, hasTimeout bool, timeout time.Duration) (Object, error) {
	futs, err := futuresOf(fs)
	if err != nil {
		return nil, err
	}
	it := &asCompletedIter{
		all:         futs,
		yielded:     map[*futureObject]bool{},
		total:       len(futs),
		hasDeadline: hasTimeout,
	}
	if hasTimeout {
		it.deadline = time.Now().Add(timeout)
	}
	// The futures already done at call time are yielded first, in input order.
	for _, f := range futs {
		if f.completedForWait() {
			it.buffer = append(it.buffer, f)
			it.yielded[f] = true
		}
	}
	return it, nil
}

func (m *asCompletedIter) TypeName() string           { return "generator" }
func (m *asCompletedIter) Iterate() (Iterator, error) { return m, nil }

// pendingCount reports how many watched futures have not yet been yielded.
func (m *asCompletedIter) pendingCount() int {
	n := 0
	for _, f := range m.all {
		if !m.yielded[f] {
			n++
		}
	}
	return n
}

// collectNewlyDone moves futures that have finished since the last scan into the
// buffer, in input order, so a wake that completes several at once yields them
// deterministically rather than in goroutine-scheduling order.
func (m *asCompletedIter) collectNewlyDone() {
	for _, f := range m.all {
		if !m.yielded[f] && f.completedForWait() {
			m.buffer = append(m.buffer, f)
			m.yielded[f] = true
		}
	}
}

func (m *asCompletedIter) Next() (Object, bool, error) {
	for {
		m.collectNewlyDone()
		if len(m.buffer) > 0 {
			f := m.buffer[0]
			m.buffer = m.buffer[1:]
			return f, true, nil
		}
		if m.pendingCount() == 0 {
			return nil, false, nil
		}
		if m.hasDeadline && !time.Now().Before(m.deadline) {
			return nil, false, Raise("TimeoutError", "%d (of %d) futures unfinished", m.pendingCount(), m.total)
		}
		// Park until another future finishes, skipping the ones already yielded so
		// the wait watches only the still-pending futures, then re-scan.
		parkForCompletion(m.all, m.yieldedSlice(), m.hasDeadline, m.deadline)
	}
}

// yieldedSlice lists the futures already handed out, the set parkForCompletion
// skips so it parks on the still-pending futures.
func (m *asCompletedIter) yieldedSlice() []*futureObject {
	var out []*futureObject
	for _, f := range m.all {
		if m.yielded[f] {
			out = append(out, f)
		}
	}
	return out
}
