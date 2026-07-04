package lower

import (
	"strings"
	"testing"
)

// Every non-generator Python frame opens with the recursion guard so a
// runaway call raises RecursionError rather than overflowing the goroutine
// stack.
func TestRecursionGuardEmitted(t *testing.T) {
	cases := map[string]string{
		"def":        "def f(n):\n    return f(n)\n",
		"method":     "class C:\n    def m(self):\n        return self.m()\n",
		"lambda":     "g = lambda n: g(n)\n",
		"nested def": "def outer():\n    def inner(n):\n        return inner(n)\n    return inner(0)\n",
	}
	for name, src := range cases {
		got, err := lowerSrc(t, src)
		if err != nil {
			t.Fatalf("%s: lower: %v", name, err)
		}
		if strings.Count(got, "runtime.EnterRecursive()") == 0 {
			t.Errorf("%s: emitted source missing recursion guard:\n%s", name, got)
		}
		if !strings.Contains(got, "defer runtime.LeaveRecursive()") {
			t.Errorf("%s: emitted source missing deferred release:\n%s", name, got)
		}
	}
}

// A generator body runs on its own goroutine, so its outer constructor does
// not carry the guard.
func TestGeneratorSkipsRecursionGuard(t *testing.T) {
	got, err := lowerSrc(t, "def g():\n    yield 1\n")
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	if strings.Contains(got, "runtime.EnterRecursive()") {
		t.Errorf("generator constructor should not charge recursion:\n%s", got)
	}
}
