package objects

// handledStack tracks the exception each active except block is handling,
// innermost last. The emitter brackets every handler body with PushHandledExc
// and PopHandledExc, and bare raise and implicit context chaining read the
// top. It lives here rather than in pkg/runtime because a generator suspends
// and resumes mid-handler: the entries its body pushed must leave the stack
// while the consumer runs and come back on resume, the way CPython stashes
// gi_exc_state per generator. Emitted programs are single-threaded until M5,
// and the generator handoff is strict ping-pong, so a plain slice is enough.
var handledStack []*Exception

// PushHandledExc records err as the exception now being handled. A non
// exception err pushes a nil slot so PopHandledExc stays balanced.
func PushHandledExc(err error) {
	e, _ := err.(*Exception)
	handledStack = append(handledStack, e)
}

// PopHandledExc drops the innermost handled exception.
func PopHandledExc() {
	if n := len(handledStack); n > 0 {
		handledStack = handledStack[:n-1]
	}
}

// CurrentHandled returns the exception being handled right now, or nil when
// no handler is active or the innermost slot holds a non-Python error.
func CurrentHandled() *Exception {
	if n := len(handledStack); n > 0 {
		return handledStack[n-1]
	}
	return nil
}

// HandledLen reports the handled-stack depth, so a test can verify the
// bracketing stays balanced and reset between cases.
func HandledLen() int {
	return len(handledStack)
}

// pushHandledSegment restores a generator's stashed handler entries on top of
// the consumer's stack as its body resumes, returning the boundary for the
// matching cut. The consumer's own entries stay beneath, so a raise inside
// the generator chains onto whatever the caller is handling, like CPython.
func pushHandledSegment(seg []*Exception) int {
	base := len(handledStack)
	handledStack = append(handledStack, seg...)
	return base
}

// cutHandledSegment removes everything the generator body left above base as
// it suspends and hands it back for stashing until the next resume. A body
// that finished has balanced its pushes, so the cut is empty then.
func cutHandledSegment(base int) []*Exception {
	if base >= len(handledStack) {
		return nil
	}
	seg := append([]*Exception(nil), handledStack[base:]...)
	handledStack = handledStack[:base]
	return seg
}
