package runtime

import (
	"fmt"
	"io"
	"strings"

	"github.com/tamnd/unagi/pkg/objects"
)

// handledStack tracks the exception each active except block is handling,
// innermost last. The emitter calls PushHandled on entry to a handler and
// PopHandled on every exit path. A plain package-level slice is enough:
// emitted programs are single-threaded until M5 brings goroutines.
var handledStack []*objects.Exception

// IsExc reports whether err is a Python-level exception.
func IsExc(err error) bool {
	_, ok := err.(*objects.Exception)
	return ok
}

// ExcObj returns the exception object behind err for `except ... as e`
// binding, or nil when err is not a Python exception.
func ExcObj(err error) objects.Object {
	if e, ok := err.(*objects.Exception); ok {
		return e
	}
	return nil
}

// ExcMatches reports whether err is an exception matching any of the
// given classes. Non-exception errors match nothing.
func ExcMatches(err error, classes ...string) bool {
	e, ok := err.(*objects.Exception)
	if !ok {
		return false
	}
	for _, c := range classes {
		if objects.Matches(e.Kind, c) {
			return true
		}
	}
	return false
}

// PushHandled records err as the exception now being handled. A non
// exception err pushes a nil slot so PopHandled stays balanced.
func PushHandled(err error) {
	e, _ := err.(*objects.Exception)
	handledStack = append(handledStack, e)
}

// PopHandled drops the innermost handled exception.
func PopHandled() {
	if n := len(handledStack); n > 0 {
		handledStack = handledStack[:n-1]
	}
}

// NewExc constructs an exception object without raising it, the
// ExceptionClass(args...) expression.
func NewExc(class string, args []objects.Object) objects.Object {
	return objects.NewException(class, args)
}

// chainInto sets newer.Context = pending following CPython's
// PyErr_SetObject rules: never link an exception to itself, and break
// any link back to newer inside pending's context chain first so no
// cycle forms. Probed on 3.14: re-raising the handled exception leaves
// its context alone, and re-raising an outer pending exception inside a
// nested handler unlinks the inner exception's back-reference.
func chainInto(newer, pending *objects.Exception) {
	if newer == nil || pending == nil || newer == pending {
		return
	}
	for o := pending; o.Context != nil; o = o.Context {
		if o.Context == newer {
			o.Context = nil
			break
		}
	}
	newer.Context = pending
}

// RaiseObj raises an exception object: `raise e`. Non-exception values
// get CPython's TypeError. The implicit context comes from the top of
// the handled stack.
func RaiseObj(o objects.Object) error {
	e, ok := o.(*objects.Exception)
	if !ok {
		// Probed on 3.14: raise 42.
		return objects.Raise(objects.TypeError, "exceptions must derive from BaseException")
	}
	// An explicit `raise e` unwinds normally, so every frame including
	// the raise line lands in the traceback. Only a bare raise skips.
	e.Reraised = false
	if len(handledStack) > 0 {
		chainInto(e, handledStack[len(handledStack)-1])
	}
	return e
}

// RaiseBare re-raises the exception being handled: bare `raise`.
// Probed on 3.14: with nothing active the message is exactly
// "No active exception to reraise" (no hyphen), and the re-raised
// traceback keeps the original raise-site line for the re-raising
// function without adding an entry for the bare raise itself, so the
// Reraised flag tells TB to skip exactly one frame.
func RaiseBare() error {
	if n := len(handledStack); n > 0 {
		if e := handledStack[n-1]; e != nil {
			e.Reraised = true
			return e
		}
	}
	return objects.Raise(objects.RuntimeError, "No active exception to reraise")
}

// SetCause implements `raise X from Y`. from None clears the cause and
// suppresses the context; an exception cause is recorded and also
// suppresses the context; anything else is CPython's TypeError.
func SetCause(err error, cause objects.Object, fromNone bool) error {
	e, ok := err.(*objects.Exception)
	if !ok {
		return err
	}
	if fromNone {
		e.Cause = nil
		e.SuppressContext = true
		return e
	}
	c, ok := cause.(*objects.Exception)
	if !ok {
		// Probed on 3.14: raise ValueError("x") from 42.
		return objects.Raise(objects.TypeError, "exception causes must derive from BaseException")
	}
	e.Cause = c
	e.SuppressContext = true
	return e
}

// ChainContext links a pending exception under a newer one, the case
// where a finally block raises over an in-flight exception. Same
// self-reference rules as the implicit raise chaining.
func ChainContext(newer, pending error) error {
	ne, ok1 := newer.(*objects.Exception)
	pe, ok2 := pending.(*objects.Exception)
	if ok1 && ok2 {
		chainInto(ne, pe)
	}
	return newer
}

// TB appends one traceback frame as err unwinds through a Python frame.
// The emitter guarantees exactly one call per frame per unwind. A bare
// re-raise consumes its Reraised flag here instead of appending, which
// reproduces the 3.14 rendering where the re-raising function shows its
// original raise-site line only.
func TB(err error, file string, line int, fn string) error {
	e, ok := err.(*objects.Exception)
	if !ok {
		return err
	}
	if e.Reraised {
		e.Reraised = false
		return e
	}
	e.Frames = append(e.Frames, objects.Frame{File: file, Line: line, Func: fn})
	return e
}

// srcLines is the compiled program's Python source split into lines,
// registered by the generated main so frame lines can quote their
// source. M1 compiles a single file, so one slice is enough. Nil means
// no source was embedded and frames render bare, which matches what
// CPython prints when the source file is gone.
var srcLines []string

// SetSource registers the embedded Python source for excerpt rendering.
func SetSource(src string) {
	srcLines = strings.Split(src, "\n")
}

// srcLine returns the stripped text of a 1-based source line, or ""
// when the line is out of range or blank. Blank excerpts print nothing,
// CPython's `if line:` guard in traceback.py.
func srcLine(n int) string {
	if n < 1 || n > len(srcLines) {
		return ""
	}
	return strings.TrimSpace(srcLines[n-1])
}

// PrintUncaught writes the CPython-3.14-shaped report for an uncaught
// exception to Stderr. Causes and contexts render first, depth first,
// with CPython's connective lines. Frame lines carry a source excerpt
// when the binary embeds its source; caret and anchor lines never
// appear because compiled code does not track columns.
func PrintUncaught(err error) {
	var b strings.Builder
	if e, ok := err.(*objects.Exception); ok {
		renderChain(&b, e, map[*objects.Exception]bool{})
	} else {
		// Not a Python exception; keep the old minimal shape.
		fmt.Fprintf(&b, "Traceback (most recent call last):\n%s\n", err.Error())
	}
	// Nothing sensible to do if stderr itself is gone.
	_, _ = io.WriteString(Stderr, b.String())
}

// renderChain renders e preceded by its cause or context. A cause wins
// over a context (an explicit cause always suppresses the context).
// The seen map guards against Cause/Context cycles.
func renderChain(b *strings.Builder, e *objects.Exception, seen map[*objects.Exception]bool) {
	seen[e] = true
	if e.Cause != nil && !seen[e.Cause] {
		renderChain(b, e.Cause, seen)
		b.WriteString("\nThe above exception was the direct cause of the following exception:\n\n")
	} else if e.Cause == nil && e.Context != nil && !e.SuppressContext && !seen[e.Context] {
		renderChain(b, e.Context, seen)
		b.WriteString("\nDuring handling of the above exception, another exception occurred:\n\n")
	}
	renderOne(b, e)
}

// renderOne renders a single exception: the traceback header and frame
// lines when there are frames (a never-raised cause has none, and 3.14
// prints it as just its final line), then the Error() line. Frames are
// stored innermost first, so they print in reverse.
func renderOne(b *strings.Builder, e *objects.Exception) {
	if len(e.Frames) > 0 {
		b.WriteString("Traceback (most recent call last):\n")
		for i := len(e.Frames) - 1; i >= 0; i-- {
			f := e.Frames[i]
			fmt.Fprintf(b, "  File %q, line %d, in %s\n", f.File, f.Line, f.Func)
			if l := srcLine(f.Line); l != "" {
				b.WriteString("    " + l + "\n")
			}
		}
	}
	b.WriteString(e.Error())
	b.WriteByte('\n')
	// PEP 678 notes print verbatim after the final line, one per line,
	// multi-line notes spanning as many lines as they contain.
	for _, n := range e.Notes {
		b.WriteString(n)
		b.WriteByte('\n')
	}
}
