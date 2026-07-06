package runtime

import (
	"bytes"
	"errors"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// Every expected message and rendering below was probed against
// python3.14 (3.14.6). Notable probe results: the bare-raise message is
// "No active exception to reraise" (no hyphen), and traceback frame
// lines carry no source excerpt when the source file is unavailable.

func resetHandled(t *testing.T) {
	t.Helper()
	clearHandled()
	t.Cleanup(clearHandled)
}

func clearHandled() {
	for objects.HandledLen() > 0 {
		objects.PopHandledExc()
	}
}

func TestIsExcAndExcObj(t *testing.T) {
	exc := objects.Raise(objects.ValueError, "boom")
	plain := errors.New("io broke")

	if !IsExc(exc) {
		t.Error("IsExc(exception) = false")
	}
	if IsExc(plain) {
		t.Error("IsExc(plain error) = true")
	}
	if o := ExcObj(exc); o != objects.Object(exc) {
		t.Errorf("ExcObj = %v, want the exception itself", o)
	}
	if o := ExcObj(plain); o != nil {
		t.Errorf("ExcObj(plain) = %v, want nil", o)
	}
}

func TestExcMatches(t *testing.T) {
	zde := objects.Raise(objects.ZeroDivisionError, "division by zero")
	if !ExcMatches(zde, "ZeroDivisionError") {
		t.Error("exact class does not match")
	}
	if !ExcMatches(zde, "ArithmeticError") || !ExcMatches(zde, "Exception") || !ExcMatches(zde, "BaseException") {
		t.Error("base classes do not match")
	}
	if ExcMatches(zde, "LookupError") {
		t.Error("unrelated class matches")
	}
	if !ExcMatches(zde, "LookupError", "ArithmeticError") {
		t.Error("multi-class tuple form does not match")
	}
	if ExcMatches(errors.New("nope"), "Exception") {
		t.Error("plain error matches a class")
	}
}

func TestNewExcAndRaiseObj(t *testing.T) {
	resetHandled(t)

	o := NewExc("ValueError", []objects.Object{objects.NewStr("x")})
	if o.TypeName() != "ValueError" {
		t.Errorf("NewExc type = %s", o.TypeName())
	}
	err := RaiseObj(o)
	if err != o.(*objects.Exception) {
		t.Errorf("RaiseObj returned %v", err)
	}

	// Probed: raise 42 -> TypeError with this exact message.
	err = RaiseObj(objects.NewInt(42))
	checkErr(t, "raise non-exception", err, "TypeError: exceptions must derive from BaseException")
}

func TestRaiseObjContextStamping(t *testing.T) {
	resetHandled(t)

	pending := objects.Raise(objects.KeyError, "k")
	PushHandled(pending)

	e := objects.NewException("ValueError", []objects.Object{objects.NewStr("v")})
	err := RaiseObj(e)
	got := err.(*objects.Exception)
	if got.Context != pending {
		t.Errorf("Context = %v, want the pending exception", got.Context)
	}

	// Re-raising the handled exception itself must not self-reference.
	// Probed: raise e inside its own handler leaves __context__ None.
	err = RaiseObj(pending)
	if err.(*objects.Exception).Context != nil {
		t.Error("self re-raise picked up a context")
	}

	// Probed cycle rule: raising the outer pending exception inside an
	// inner handler unlinks the inner exception's back-reference, so
	// e.__context__ is the inner one and the inner one's context is None.
	PushHandled(e) // e.Context == pending from above
	err = RaiseObj(pending)
	outer := err.(*objects.Exception)
	if outer.Context != e {
		t.Errorf("outer.Context = %v, want inner", outer.Context)
	}
	if e.Context != nil {
		t.Error("cycle not broken: inner still links back")
	}
	PopHandled()
	PopHandled()
}

func TestRaiseBare(t *testing.T) {
	resetHandled(t)

	// Probed: bare raise with nothing active, exact 3.14 wording.
	err := RaiseBare()
	checkErr(t, "empty stack", err, "RuntimeError: No active exception to reraise")

	exc := objects.Raise(objects.ValueError, "boom")
	PushHandled(exc)
	err = RaiseBare()
	if err != error(exc) {
		t.Errorf("RaiseBare = %v, want the handled exception", err)
	}
	PopHandled()
}

func TestSetCause(t *testing.T) {
	e := objects.Raise(objects.ValueError, "x")
	cause := objects.Raise(objects.KeyError, "y")

	err := SetCause(e, cause, false)
	if err != error(e) || e.Cause != cause || !e.SuppressContext {
		t.Errorf("SetCause exception form: err=%v cause=%v suppress=%v", err, e.Cause, e.SuppressContext)
	}

	// from None clears the cause and still suppresses the context.
	e2 := objects.Raise(objects.ValueError, "x")
	e2.Context = cause
	err = SetCause(e2, objects.None, true)
	if err != error(e2) || e2.Cause != nil || !e2.SuppressContext || e2.Context != cause {
		t.Errorf("SetCause from None: cause=%v suppress=%v context=%v", e2.Cause, e2.SuppressContext, e2.Context)
	}

	// Probed: raise ValueError("x") from 42 -> this exact TypeError.
	err = SetCause(objects.Raise(objects.ValueError, "x"), objects.NewInt(42), false)
	checkErr(t, "non-exception cause", err, "TypeError: exception causes must derive from BaseException")
}

func TestChainContext(t *testing.T) {
	pending := objects.Raise(objects.KeyError, "k")
	newer := objects.Raise(objects.ValueError, "v")

	if got := ChainContext(newer, pending); got != error(newer) {
		t.Errorf("ChainContext returned %v", got)
	}
	if newer.Context != pending {
		t.Errorf("Context = %v", newer.Context)
	}

	// Chaining an exception under itself is a no-op.
	same := objects.Raise(objects.ValueError, "s")
	_ = ChainContext(same, same)
	if same.Context != nil {
		t.Error("self chain set a context")
	}

	// A link back to newer inside pending's chain gets cut first.
	a := objects.Raise(objects.ValueError, "a")
	b := objects.Raise(objects.KeyError, "b")
	b.Context = a
	_ = ChainContext(a, b)
	if a.Context != b || b.Context != nil {
		t.Errorf("cycle guard: a.Context=%v b.Context=%v", a.Context, b.Context)
	}

	// Non-exception errors pass through untouched.
	plain := errors.New("plain")
	if got := ChainContext(plain, pending); got != plain {
		t.Errorf("plain ChainContext = %v", got)
	}
}

func TestTBOrderAndBareReraiseSkip(t *testing.T) {
	resetHandled(t)

	// Normal unwind: one frame per Python frame, innermost first.
	err := RaiseObj(NewExc("ValueError", []objects.Object{objects.NewStr("boom")}))
	err = TB(err, "main.py", 2, "inner")
	err = TB(err, "main.py", 6, "middle")
	e := err.(*objects.Exception)
	if len(e.Frames) != 2 || e.Frames[0].Func != "inner" || e.Frames[1].Func != "middle" {
		t.Fatalf("Frames = %+v", e.Frames)
	}

	// Bare re-raise inside middle's handler. Probed on 3.14: the
	// traceback keeps middle's original call-site line and gains no
	// entry for the bare raise line, then callers stack on top.
	PushHandled(err)
	err = RaiseBare()
	err = TB(err, "main.py", 8, "middle") // the raise line: skipped
	PopHandled()
	err = TB(err, "main.py", 10, "<module>")
	e = err.(*objects.Exception)
	if len(e.Frames) != 3 {
		t.Fatalf("after re-raise Frames = %+v", e.Frames)
	}
	if e.Frames[1].Line != 6 || e.Frames[2].Func != "<module>" || e.Frames[2].Line != 10 {
		t.Errorf("re-raise frame shape = %+v", e.Frames)
	}

	// An explicit `raise e` does add a frame for the raise line,
	// probed: middle shows line 8 (raise e) above line 6 (the call).
	resetHandled(t)
	err = RaiseObj(NewExc("ValueError", []objects.Object{objects.NewStr("boom")}))
	err = TB(err, "main.py", 2, "inner")
	err = TB(err, "main.py", 6, "middle")
	PushHandled(err)
	err = RaiseObj(ExcObj(err))
	err = TB(err, "main.py", 8, "middle")
	PopHandled()
	err = TB(err, "main.py", 10, "<module>")
	e = err.(*objects.Exception)
	if len(e.Frames) != 4 || e.Frames[2].Line != 8 {
		t.Errorf("raise e frame shape = %+v", e.Frames)
	}

	// TB passes non-exception errors through.
	plain := errors.New("plain")
	if got := TB(plain, "f", 1, "g"); got != plain {
		t.Errorf("TB(plain) = %v", got)
	}
}

func captureUncaught(t *testing.T, err error) string {
	t.Helper()
	var buf bytes.Buffer
	old := Stderr
	Stderr = &buf
	defer func() { Stderr = old }()
	PrintUncaught(err)
	return buf.String()
}

// The golden strings are CPython 3.14 stderr for the equivalent
// programs, minus the source excerpt and caret lines. Compiled binaries
// do not embed source, and CPython prints the same File-line-only shape
// when the .py has been deleted (probed by compiling to .pyc, removing
// the source and running it).

func TestPrintUncaughtSingle(t *testing.T) {
	e := objects.NewException("ValueError", []objects.Object{objects.NewStr("deep failure")})
	e.Frames = []objects.Frame{
		{File: "main.py", Line: 2, Func: "inner"},
		{File: "main.py", Line: 6, Func: "<module>"},
	}
	want := "Traceback (most recent call last):\n" +
		"  File \"main.py\", line 6, in <module>\n" +
		"  File \"main.py\", line 2, in inner\n" +
		"ValueError: deep failure\n"
	if got := captureUncaught(t, e); got != want {
		t.Errorf("single = %q, want %q", got, want)
	}
}

func TestPrintUncaughtContextChain(t *testing.T) {
	inner := objects.NewException("KeyError", []objects.Object{objects.NewStr("inner")})
	inner.Frames = []objects.Frame{{File: "main.py", Line: 2, Func: "<module>"}}
	outer := objects.NewException("ValueError", []objects.Object{objects.NewStr("outer")})
	outer.Frames = []objects.Frame{{File: "main.py", Line: 4, Func: "<module>"}}
	outer.Context = inner

	want := "Traceback (most recent call last):\n" +
		"  File \"main.py\", line 2, in <module>\n" +
		"KeyError: 'inner'\n" +
		"\n" +
		"During handling of the above exception, another exception occurred:\n" +
		"\n" +
		"Traceback (most recent call last):\n" +
		"  File \"main.py\", line 4, in <module>\n" +
		"ValueError: outer\n"
	if got := captureUncaught(t, outer); got != want {
		t.Errorf("context chain = %q, want %q", got, want)
	}
}

func TestPrintUncaughtCauseChain(t *testing.T) {
	cause := objects.NewException("KeyError", []objects.Object{objects.NewStr("inner")})
	cause.Frames = []objects.Frame{
		{File: "main.py", Line: 2, Func: "a"},
		{File: "main.py", Line: 6, Func: "b"},
	}
	outer := objects.NewException("ValueError", []objects.Object{objects.NewStr("outer")})
	outer.Frames = []objects.Frame{
		{File: "main.py", Line: 8, Func: "b"},
		{File: "main.py", Line: 10, Func: "<module>"},
	}
	outer.Cause = cause
	outer.Context = cause
	outer.SuppressContext = true

	want := "Traceback (most recent call last):\n" +
		"  File \"main.py\", line 6, in b\n" +
		"  File \"main.py\", line 2, in a\n" +
		"KeyError: 'inner'\n" +
		"\n" +
		"The above exception was the direct cause of the following exception:\n" +
		"\n" +
		"Traceback (most recent call last):\n" +
		"  File \"main.py\", line 10, in <module>\n" +
		"  File \"main.py\", line 8, in b\n" +
		"ValueError: outer\n"
	if got := captureUncaught(t, outer); got != want {
		t.Errorf("cause chain = %q, want %q", got, want)
	}
}

func TestPrintUncaughtFromNone(t *testing.T) {
	ctx := objects.NewException("ZeroDivisionError", []objects.Object{objects.NewStr("division by zero")})
	ctx.Frames = []objects.Frame{{File: "main.py", Line: 2, Func: "<module>"}}
	e := objects.NewException("RuntimeError", []objects.Object{objects.NewStr("no context shown")})
	e.Frames = []objects.Frame{{File: "main.py", Line: 4, Func: "<module>"}}
	e.Context = ctx
	e.SuppressContext = true

	want := "Traceback (most recent call last):\n" +
		"  File \"main.py\", line 4, in <module>\n" +
		"RuntimeError: no context shown\n"
	if got := captureUncaught(t, e); got != want {
		t.Errorf("from None = %q, want %q", got, want)
	}
}

func TestPrintUncaughtTracebackLessCause(t *testing.T) {
	// Probed: raise ValueError("outer") from KeyError("y") renders the
	// never-raised cause as just its final line, no header.
	cause := objects.NewException("KeyError", []objects.Object{objects.NewStr("y")})
	e := objects.NewException("ValueError", []objects.Object{objects.NewStr("outer")})
	e.Frames = []objects.Frame{{File: "main.py", Line: 1, Func: "<module>"}}
	e.Cause = cause
	e.SuppressContext = true

	want := "KeyError: 'y'\n" +
		"\n" +
		"The above exception was the direct cause of the following exception:\n" +
		"\n" +
		"Traceback (most recent call last):\n" +
		"  File \"main.py\", line 1, in <module>\n" +
		"ValueError: outer\n"
	if got := captureUncaught(t, e); got != want {
		t.Errorf("traceback-less cause = %q, want %q", got, want)
	}
}

func TestPrintUncaughtCycleGuard(t *testing.T) {
	a := objects.NewException("ValueError", []objects.Object{objects.NewStr("a")})
	b := objects.NewException("KeyError", []objects.Object{objects.NewStr("b")})
	a.Context = b
	b.Context = a // never built by the chaining helpers, but must not hang

	got := captureUncaught(t, a)
	if got == "" {
		t.Fatal("cycle rendered nothing")
	}
	if want := "ValueError: a\n"; !bytes.HasSuffix([]byte(got), []byte(want)) {
		t.Errorf("cycle output = %q, want suffix %q", got, want)
	}
}
