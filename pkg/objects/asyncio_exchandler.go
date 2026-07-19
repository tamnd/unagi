package objects

import (
	"fmt"
	"sort"
	"strings"
)

// The event loop's exception-handler surface (asyncio/base_events.py). A loop
// starts with no custom handler and reports errors through
// default_exception_handler; set_exception_handler installs a callable that
// call_exception_handler routes each error context to instead, and passing None
// restores the default. get_exception_handler reads the current one back, None
// while the default is in force.

// stderrWrite routes runtime diagnostics that CPython sends to stderr, like the
// event loop's default exception handler, through the host's stderr sink. Runtime
// wires it at init; a call before wiring, or with no runtime, writes nothing.
var stderrWrite func(string)

// SetStderrWrite installs the sink the loop's default_exception_handler writes to.
func SetStderrWrite(w func(string)) { stderrWrite = w }

// setExceptionHandler implements loop.set_exception_handler(handler). CPython
// accepts a callable or None; None clears back to the default, and anything else
// is the TypeError with the argument's repr.
func (l *eventLoop) setExceptionHandler(handler Object) (Object, error) {
	if handler == None {
		l.exceptionHandler = nil
		return None, nil
	}
	if !Callable(handler) {
		return nil, Raise(TypeError, "A callable object or None is expected, got %s", Repr(handler))
	}
	l.exceptionHandler = handler
	return None, nil
}

// callExceptionHandler implements loop.call_exception_handler(context). With no
// custom handler set it runs the default handler; with one set it calls it as
// handler(loop, context). CPython never lets an error escape this call: if the
// custom handler itself raises, it falls back to the default handler with a
// wrapper context. That fallback still runs the default handler's message lines;
// the exc_info traceback for the wrapped exception is deferred with the rest of
// default_exception_handler's traceback rendering.
func (l *eventLoop) callExceptionHandler(t *Thread, context Object) (Object, error) {
	if l.exceptionHandler == nil {
		return l.defaultExceptionHandler(context)
	}
	if _, err := CallT(t, l.exceptionHandler, []Object{l, context}); err != nil {
		exc := errorObject(err)
		wrap, werr := NewDict(
			[]Object{NewStr("message"), NewStr("exception"), NewStr("context")},
			[]Object{NewStr("Unhandled error in exception handler"), exc, context},
		)
		if werr != nil {
			return nil, werr
		}
		return l.defaultExceptionHandler(wrap)
	}
	return None, nil
}

// defaultExceptionHandler implements loop.default_exception_handler(context). It
// logs the context's 'message' (or a stock message when absent or empty),
// followed by every other key sorted, each rendered "key: repr(value)", the way
// CPython builds log_lines before handing them to the asyncio logger. The
// 'exception' key is skipped here as it is in CPython, where it feeds the log's
// exc_info instead; rendering that traceback is deferred, so a context carrying an
// exception logs the same lines without the trailing traceback.
func (l *eventLoop) defaultExceptionHandler(context Object) (Object, error) {
	d, ok := context.(*dictObject)
	if !ok {
		return None, nil
	}
	message := "Unhandled exception in event loop"
	if msg, found, err := d.lookup(NewStr("message")); err != nil {
		return nil, err
	} else if found && Truth(msg) {
		message = Str(msg)
	}
	lines := []string{message}
	var keys []string
	for _, k := range d.keySlice() {
		s, ok := k.(*strObject)
		if !ok {
			continue
		}
		if s.v == "message" || s.v == "exception" {
			continue
		}
		keys = append(keys, s.v)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v, _, err := d.lookup(NewStr(k))
		if err != nil {
			return nil, err
		}
		lines = append(lines, fmt.Sprintf("%s: %s", k, Repr(v)))
	}
	if stderrWrite != nil {
		stderrWrite(strings.Join(lines, "\n") + "\n")
	}
	return None, nil
}
