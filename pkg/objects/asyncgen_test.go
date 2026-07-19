package objects

import (
	"testing"
)

// advanceAsyncGen drives an async generator one __anext__ step from inside a
// coroutine body, discarding the yielded value. It is the Go-side spelling of
// `await g.__anext__()`.
func advanceAsyncGen(y Yielder, g Object) error {
	aw, err := CallMethod(g, "__anext__", nil)
	if err != nil {
		return err
	}
	_, err = awaitObj(y, aw)
	return err
}

// acloseAsyncGen drives `await g.aclose()` from inside a coroutine body and
// hands back its result, which is None on a clean close.
func acloseAsyncGen(y Yielder, g Object) (Object, error) {
	aw, err := CallMethod(g, "aclose", nil)
	if err != nil {
		return nil, err
	}
	return awaitObj(y, aw)
}

// TestAsyncGenACloseCleanReturnsNone closes a started async generator whose body
// lets GeneratorExit propagate and checks the await result is None.
func TestAsyncGenACloseCleanReturnsNone(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		g := NewAsyncGenerator("gen", func(gy Yielder) (Object, error) {
			if _, err := gy.Yield(NewInt(1)); err != nil {
				return nil, err
			}
			if _, err := gy.Yield(NewInt(2)); err != nil {
				return nil, err
			}
			return None, nil
		})
		if err := advanceAsyncGen(y, g); err != nil {
			return nil, err
		}
		return acloseAsyncGen(y, g)
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got != None {
		t.Fatalf("aclose result = %v, want None", Repr(got))
	}
}

// TestAsyncGenACloseRunsFinally closes an async generator whose body has a
// finally that awaits, and checks the finally ran during the close.
func TestAsyncGenACloseRunsFinally(t *testing.T) {
	ran := false
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		g := NewAsyncGenerator("gen", func(gy Yielder) (Object, error) {
			defer func() { ran = true }()
			if _, err := gy.Yield(NewInt(1)); err != nil {
				// GeneratorExit surfaces here; forward the inner await, then let it
				// propagate so the close completes cleanly.
				if _, aerr := awaitObj(gy, AsyncioSleep(0, None)); aerr != nil {
					return nil, aerr
				}
				return nil, err
			}
			return None, nil
		})
		if err := advanceAsyncGen(y, g); err != nil {
			return nil, err
		}
		return acloseAsyncGen(y, g)
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !ran {
		t.Fatal("finally did not run during aclose")
	}
}

// TestAsyncGenACloseIgnoredIsRuntimeError closes an async generator that catches
// GeneratorExit and yields again, and checks the RuntimeError CPython raises.
func TestAsyncGenACloseIgnoredIsRuntimeError(t *testing.T) {
	var closeErr error
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		g := NewAsyncGenerator("gen", func(gy Yielder) (Object, error) {
			v, err := gy.Yield(NewInt(1))
			if err != nil {
				// Swallow the injected GeneratorExit and keep producing, the bug the
				// close is meant to surface.
				if _, yerr := gy.Yield(NewInt(99)); yerr != nil {
					return nil, yerr
				}
				return None, nil
			}
			_ = v
			return None, nil
		})
		if err := advanceAsyncGen(y, g); err != nil {
			return nil, err
		}
		_, closeErr = acloseAsyncGen(y, g)
		return None, nil
	})
	if _, err := AsyncioRun(main); err != nil {
		t.Fatalf("run: %v", err)
	}
	if coroExcKind(closeErr) != "RuntimeError" {
		t.Fatalf("aclose of ignoring gen = %v, want RuntimeError", closeErr)
	}
}

// TestAsyncGenACloseNeverStartedReturnsNone closes an async generator that was
// never advanced and checks the body never ran and the result is None.
func TestAsyncGenACloseNeverStartedReturnsNone(t *testing.T) {
	ran := false
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		g := NewAsyncGenerator("gen", func(gy Yielder) (Object, error) {
			ran = true
			if _, err := gy.Yield(NewInt(1)); err != nil {
				return nil, err
			}
			return None, nil
		})
		return acloseAsyncGen(y, g)
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got != None {
		t.Fatalf("aclose result = %v, want None", Repr(got))
	}
	if ran {
		t.Fatal("never-started aclose ran the body")
	}
}
