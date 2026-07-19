package objects

import (
	"testing"
)

// coroExcKind reports the exception kind an asyncio helper returned, or "" when
// the error is not an Exception.
func coroExcKind(err error) string {
	if e, ok := err.(*Exception); ok {
		return e.Kind
	}
	return ""
}

// TestAsyncioRunReturnsResult drives a coroutine that awaits a child coroutine
// and a sleep, then returns a value, and checks run hands the value back.
func TestAsyncioRunReturnsResult(t *testing.T) {
	child := func() Object {
		return NewCoroutine("child", func(y Yielder) (Object, error) {
			if _, err := y.YieldFrom(AsyncioSleep(0, None)); err != nil {
				return nil, err
			}
			return NewInt(7), nil
		})
	}
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		v, err := y.YieldFrom(child())
		if err != nil {
			return nil, err
		}
		return v, nil
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if n, ok := AsInt(got); !ok || n != 7 {
		t.Fatalf("run returned %v, want 7", Repr(got))
	}
}

// TestAsyncioSleepResult checks sleep hands back its result argument.
func TestAsyncioSleepResult(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		return y.YieldFrom(AsyncioSleep(0, NewStr("done")))
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if s, ok := AsStr(got); !ok || s != "done" {
		t.Fatalf("sleep result %v, want 'done'", Repr(got))
	}
}

// TestAsyncioSleepTimer checks a positive delay resolves through a timer and the
// coroutine resumes after it.
func TestAsyncioSleepTimer(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		if _, err := y.YieldFrom(AsyncioSleep(0.005, None)); err != nil {
			return nil, err
		}
		return NewInt(1), nil
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if n, ok := AsInt(got); !ok || n != 1 {
		t.Fatalf("run returned %v, want 1", Repr(got))
	}
}

// TestAsyncioRunNonCoroutine checks a non-coroutine argument is a TypeError.
func TestAsyncioRunNonCoroutine(t *testing.T) {
	if _, err := AsyncioRun(NewInt(123)); coroExcKind(err) != "TypeError" {
		t.Fatalf("run(int) = %v, want TypeError", err)
	}
}

// TestAsyncioRunPropagatesException checks an exception raised in the coroutine
// propagates out of run.
func TestAsyncioRunPropagatesException(t *testing.T) {
	main := NewCoroutine("boom", func(y Yielder) (Object, error) {
		return nil, Raise(ValueError, "kaboom")
	})
	if _, err := AsyncioRun(main); coroExcKind(err) != "ValueError" {
		t.Fatalf("run(boom) = %v, want ValueError", err)
	}
}

// TestAsyncioRunNested checks that calling run inside a running loop raises the
// RuntimeError CPython raises, without disturbing the outer run.
func TestAsyncioRunNested(t *testing.T) {
	inner := NewCoroutine("inner", func(y Yielder) (Object, error) { return None, nil })
	var nested error
	outer := NewCoroutine("outer", func(y Yielder) (Object, error) {
		_, nested = AsyncioRun(inner)
		return None, nil
	})
	if _, err := AsyncioRun(outer); err != nil {
		t.Fatalf("outer run: %v", err)
	}
	if coroExcKind(nested) != "RuntimeError" {
		t.Fatalf("nested run = %v, want RuntimeError", nested)
	}
	// The inner coroutine never ran; close it so it is not left started.
	if _, err := inner.(*generatorObject).closeGen(); err != nil {
		t.Fatalf("close inner: %v", err)
	}
}

// TestAsyncioRunningLoopOutside checks there is no running loop before or after
// a run, and one during it.
func TestAsyncioRunningLoopOutside(t *testing.T) {
	if AsyncioRunningLoop() != nil {
		t.Fatalf("running loop before run = %v, want nil", AsyncioRunningLoop())
	}
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		if AsyncioRunningLoop() == nil {
			return nil, Raise(RuntimeError, "expected a running loop")
		}
		return None, nil
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
	if AsyncioRunningLoop() != nil {
		t.Fatalf("running loop after run = %v, want nil", AsyncioRunningLoop())
	}
}
