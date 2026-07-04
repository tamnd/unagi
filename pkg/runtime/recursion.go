package runtime

import "github.com/tamnd/unagi/pkg/objects"

// Python bounds recursion with a depth counter (sys.getrecursionlimit,
// default 1000 on 3.14) so a runaway call raises a catchable RecursionError
// instead of overflowing the interpreter's C stack. The emitted Go code
// recurses on the goroutine stack, which would otherwise fault with an
// uncatchable fatal error, so every Python frame charges one slot here on
// entry and releases it on exit.
//
// Like handledStack this is a package-level global tracking one logical line
// of execution; per-goroutine isolation waits on the runtime-state refactor,
// so a generator body driven on its own goroutine is not accounted here yet.
var (
	recursionLimit = 1000
	callDepth      int
)

// EnterRecursive charges one Python frame and returns a RecursionError when
// the new depth passes the limit. A frame that trips the limit never really
// runs, so it takes its charge back before returning the error, keeping the
// counter balanced without a paired LeaveRecursive.
func EnterRecursive() error {
	callDepth++
	if callDepth > recursionLimit {
		callDepth--
		return objects.Raise(objects.RecursionError, "maximum recursion depth exceeded")
	}
	return nil
}

// LeaveRecursive releases the slot as a Python frame returns or unwinds. It
// pairs with a successful EnterRecursive through a deferred call.
func LeaveRecursive() {
	if callDepth > 0 {
		callDepth--
	}
}

// SetRecursionLimit and RecursionLimit back a future sys.setrecursionlimit /
// sys.getrecursionlimit; the limit applies to the next frame charged.
func SetRecursionLimit(n int) { recursionLimit = n }

// RecursionLimit reports the current limit.
func RecursionLimit() int { return recursionLimit }
