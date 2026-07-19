package lower

import (
	"strings"
	"testing"
)

// TestAsyncComprehensionOutsideAsyncRejected checks the 3.14 SyntaxError an
// async comprehension raises when it is not inside an async def. The parser
// accepts the async-for leg and defers this to lowering, since the enclosing
// function is what decides legality.
func TestAsyncComprehensionOutsideAsyncRejected(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"list async for", "x = [i async for i in y]\n"},
		{"set async for", "x = {i async for i in y}\n"},
		{"dict async for", "x = {i: i async for i in y}\n"},
		{"second clause async", "x = [i for i in y async for j in z]\n"},
		{"await in element", "x = [await f(i) for i in y]\n"},
		{"await in condition", "x = [i for i in y if await f(i)]\n"},
		{"sync def", "def g():\n    return [i async for i in y]\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := lowerSrc(t, c.src)
			if err == nil || !strings.Contains(err.Error(), "asynchronous comprehension outside of an asynchronous function") {
				t.Fatalf("want async-comprehension error, got %v", err)
			}
		})
	}
}

// TestAsyncComprehensionInsideAsyncLowers checks an async comprehension inside
// an async def lowers to the async iterator protocol the async for loop uses,
// AsyncIterT for the iterator and AsyncNextT for each step.
func TestAsyncComprehensionInsideAsyncLowers(t *testing.T) {
	src := "async def f(y):\n    return [i async for i in y]\n"
	got, err := lowerSrc(t, src)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	for _, want := range []string{"objects.AsyncIterT(t,", "objects.AsyncNextT(t, gy,"} {
		if !strings.Contains(got, want) {
			t.Errorf("emitted source missing %q:\n%s", want, got)
		}
	}
}

// TestAwaitOnlyComprehensionInsideAsyncLowers checks a comprehension with no
// async-for clause but an await in the element is treated as async and lowers
// through the await path inside an async def rather than being rejected.
func TestAwaitOnlyComprehensionInsideAsyncLowers(t *testing.T) {
	src := "async def f(y):\n    return [await g(i) for i in y]\n"
	got, err := lowerSrc(t, src)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	if !strings.Contains(got, "gy.YieldFrom(") {
		t.Errorf("emitted source missing await lowering:\n%s", got)
	}
}

// TestAsyncGeneratorExpressionDeferred checks an async generator expression is
// refused with a clear not-supported-yet message rather than lowered over the
// sync iterator protocol, which would drive an async iterable wrong.
func TestAsyncGeneratorExpressionDeferred(t *testing.T) {
	_, err := lowerSrc(t, "async def f(y):\n    return (i async for i in y)\n")
	if err == nil || !strings.Contains(err.Error(), "asynchronous generator expression is not supported yet") {
		t.Fatalf("want async-genexp deferral, got %v", err)
	}
}
