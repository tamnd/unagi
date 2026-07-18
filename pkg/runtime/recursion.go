package runtime

import "github.com/tamnd/unagi/pkg/objects"

// Python bounds recursion with a depth counter (sys.getrecursionlimit,
// default 1000 on 3.14) so a runaway call raises a catchable RecursionError
// instead of overflowing the interpreter's C stack. The emitted Go code
// recurses on the goroutine stack, which would otherwise fault with an
// uncatchable fatal error, so every Python frame charges one slot here on
// entry and releases it on exit.
//
// The depth is per-thread, kept on the *objects.Thread the call spine already
// threads through every frame, mirroring CPython's per-tstate accounting: two
// threads each recursing deeply do not add up against one shared counter, and
// the counter carries no data race because only the owning goroutine touches
// it. The limit stays process-wide, matching sys.setrecursionlimit, and is set
// on the main thread before any second goroutine starts, so its reads race with
// nothing.
var recursionLimit = 1000

// EnterRecursive charges one Python frame against t's depth and returns a
// RecursionError when the new depth passes the limit.
func EnterRecursive(t *objects.Thread) error {
	return t.EnterRecursive(recursionLimit)
}

// LeaveRecursive releases the slot as a Python frame on t returns or unwinds. It
// pairs with a successful EnterRecursive through a deferred call.
func LeaveRecursive(t *objects.Thread) {
	t.LeaveRecursive()
}

// SetRecursionLimit and RecursionLimit back a future sys.setrecursionlimit /
// sys.getrecursionlimit; the limit applies to the next frame charged.
func SetRecursionLimit(n int) { recursionLimit = n }

// RecursionLimit reports the current limit.
func RecursionLimit() int { return recursionLimit }
