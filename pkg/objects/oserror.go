package objects

import (
	"fmt"
	"strings"
)

// OSError value semantics. CPython gives OSError a C-level constructor that,
// when handed 2 to 5 positional arguments, splits them into the named members
// errno, strerror, filename, winerror and filename2, collapses args back to the
// (errno, strerror) pair, and — when the class being built is exactly OSError —
// remaps the result to the errno-specific subclass (ENOENT -> FileNotFoundError,
// EACCES -> PermissionError, and so on). str() then renders the familiar
// "[Errno 2] No such file or directory: 'x'" form. parseOSErrorArgs reproduces
// that split for the boxed exception; osErrorText reproduces the str().
//
// winerror (args[3]) is a Windows-only slot: on POSIX it is accepted positionally
// but never surfaced as an attribute, so it is skipped here. characters_written
// is a BlockingIOError-only slot CPython leaves unset until written; unagi does
// not model it and reads of it fall through to AttributeError, matching a fresh
// BlockingIOError on POSIX.

// OSErrorSubclass maps an errno to the OSError subclass CPython selects for it,
// or reports no mapping. It is a hook so the errno constants stay in a
// platform-tagged runtime file (they come from package syscall) and this package
// stays platform-independent. It is nil until a runtime init wires it; on a host
// with no mapping wired, base OSError simply keeps its class, which is safe.
var OSErrorSubclass func(errno int64) (string, bool)

// parseOSErrorArgs performs CPython's OSError argument split on a freshly built
// exception. It runs only for the OSError family and only for the 2..5 argument
// count CPython treats specially; fewer or more arguments keep the generic
// BaseException shape (args stay whole, the slots stay None, str is the tuple).
func parseOSErrorArgs(e *Exception) {
	// The argument-count gate comes before the class check on purpose: every
	// exception construction routes through here (NewException builds KeyError,
	// StopIteration and friends), and the common 0- and 1-argument cases skip the
	// Matches graph walk entirely on the cheap integer compare.
	n := len(e.Args)
	if n < 2 || n > 5 {
		return
	}
	if !Matches(e.Kind, "OSError") {
		return
	}
	e.OSParsed = true
	e.OSErrno = e.Args[0]
	e.OSStrError = e.Args[1]
	if n >= 3 {
		e.OSFilename = e.Args[2]
	}
	if n >= 5 {
		// args[3] is winerror, dropped on POSIX; args[4] is filename2.
		e.OSFilename2 = e.Args[4]
	}
	// args collapses to the (errno, strerror) pair, so repr(e) and e.args match
	// CPython's FileNotFoundError(2, 'msg') rather than echoing the filename.
	e.Args = e.Args[:2:2]
	// Only a bare OSError remaps to a more specific subclass; a directly
	// constructed subclass (FileNotFoundError(2, ...)) keeps its own class.
	if e.Kind == "OSError" && OSErrorSubclass != nil {
		if code, ok := AsInt(e.OSErrno); ok {
			if sub, ok := OSErrorSubclass(code); ok {
				if cls, ok := ExcClass(sub); ok {
					e.Kind = sub
					e.Class = cls
				}
			}
		}
	}
}

// osErrorText renders str() for a split OSError. With no filename it is
// "[Errno N] strerror"; a filename appends ": 'name'", and a filename2 (a rename
// or link source/target pair) appends " -> 'name2'" after it, matching CPython's
// OSError_str exactly, including that filename2 shows only when filename does.
func osErrorText(e *Exception) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[Errno %s] %s", Str(e.OSErrno), Str(e.OSStrError))
	if e.OSFilename != nil && e.OSFilename != None {
		fmt.Fprintf(&b, ": %s", Repr(e.OSFilename))
		if e.OSFilename2 != nil && e.OSFilename2 != None {
			fmt.Fprintf(&b, " -> %s", Repr(e.OSFilename2))
		}
	}
	return b.String()
}

// objOrNone returns o, or None when the slot was never filled, so an unset
// OSError member reads as None the way CPython's default does.
func objOrNone(o Object) Object {
	if o == nil {
		return None
	}
	return o
}
