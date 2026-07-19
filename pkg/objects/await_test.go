package objects

import (
	"testing"
)

// TestCoroutineAwaitMethodDrives checks a coroutine exposes __await__ and that
// yield-from over the returned iterator drives the coroutine to its result, the
// delegating idiom a user awaitable uses to forward to a library coroutine.
func TestCoroutineAwaitMethodDrives(t *testing.T) {
	main := NewCoroutine("main", func(y Yielder) (Object, error) {
		inner := sleepThen("inner", 0, NewInt(7))
		aw, err := CallMethod(inner, "__await__", nil)
		if err != nil {
			return nil, err
		}
		return y.YieldFrom(aw)
	})
	got, err := AsyncioRun(main)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if n, ok := AsInt(got); !ok || n != 7 {
		t.Fatalf("coroutine __await__ result = %v, want 7", Repr(got))
	}
}

// TestGeneratorHasNoAwaitMethod checks a plain generator, which is not awaitable,
// reports no __await__ attribute.
func TestGeneratorHasNoAwaitMethod(t *testing.T) {
	gen := NewGenerator("gen", func(y Yielder) (Object, error) { return None, nil })
	if _, err := CallMethod(gen, "__await__", nil); !isAttrError(err) {
		t.Fatalf("generator __await__ = %v, want AttributeError", err)
	}
	// Close it so it is not left started, matching the other generator tests.
	if _, err := gen.(*generatorObject).closeGen(); err != nil {
		t.Fatalf("close gen: %v", err)
	}
}
