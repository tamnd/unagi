package objects

import "testing"

func TestIsCoroutineObject(t *testing.T) {
	coro := NewCoroutine("c", func(y Yielder) (Object, error) { return None, nil })
	if !IsCoroutineObject(coro) {
		t.Error("a coroutine should be a coroutine object")
	}
	// An async generator is driven through the async protocol, not awaited as a
	// coroutine, so it is not one.
	asyncGen := NewAsyncGenerator("g", func(y Yielder) (Object, error) { return None, nil })
	if IsCoroutineObject(asyncGen) {
		t.Error("an async generator is not a coroutine object")
	}
	for _, o := range []Object{NewInt(1), None, NewStr("x")} {
		if IsCoroutineObject(o) {
			t.Errorf("%s is not a coroutine object", Repr(o))
		}
	}
}

func TestIsFutureObject(t *testing.T) {
	// Build the future and task values directly, since AsyncioNewFuture wants a
	// running loop; the predicate only inspects the type.
	if !IsFutureObject(&asyncFuture{}) {
		t.Error("an asyncio Future should be a future object")
	}
	if !IsFutureObject(&asyncTask{}) {
		t.Error("an asyncio Task should be a future object")
	}
	for _, o := range []Object{NewInt(1), None, NewStr("x")} {
		if IsFutureObject(o) {
			t.Errorf("%s is not a future object", Repr(o))
		}
	}
}
