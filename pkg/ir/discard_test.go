package ir

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/emit"
)

// TestLowerDropsDocstring proves the headline case of doc 06 line 55: a function
// whose first line is a docstring still lowers to the static tier. The docstring is
// a bare string expression with no effect and no way to raise, so it drops to no
// statement at all rather than boxing the whole function, which is what a missing
// ExprStmt case did before.
func TestLowerDropsDocstring(t *testing.T) {
	src := "def sq(x: int) -> int:\n    \"return x squared\"\n    return x * x\n"
	got := emitOf(t, src)
	for _, want := range []string{
		"func sq(x int64) (int64, error)",
		"rt.MulInt64(x, x)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("docstring function did not lower static, missing %q:\n%s", want, got)
		}
	}
	// The docstring text never reaches the emitted Go: it is dropped, not bound to a
	// blank, so no dead string literal survives.
	if strings.Contains(got, "return x squared") {
		t.Errorf("the docstring should be dropped, not emitted:\n%s", got)
	}
}

// TestLowerDropsBareConstantAndName proves the other pure-discardable forms drop
// the same way a docstring does: a bare integer on a line and a bare name read have
// no effect and cannot raise once their value is thrown away, so neither emits a
// statement while the function still lowers.
func TestLowerDropsBareConstantAndName(t *testing.T) {
	src := "def g(x: int) -> int:\n    42\n    x\n    return x\n"
	got := emitOf(t, src)
	if !strings.Contains(got, "func g(x int64) (int64, error)") {
		t.Fatalf("function with bare discardable lines did not lower static:\n%s", got)
	}
	// Neither the bare constant nor the bare name leaves a stray statement in the body.
	if strings.Contains(got, "_ = 42") || strings.Contains(got, "_ = x") {
		t.Errorf("a bare pure value should be dropped, not bound to the blank:\n%s", got)
	}
}

// TestLowerDiscardsCallForEffect proves a call on a line by itself keeps its effect
// and its D14 error even though the value is unused: it lowers to a static call
// bound to the blank with the callee's error threaded to the caller's zero return,
// so an exception the callee raises still propagates.
func TestLowerDiscardsCallForEffect(t *testing.T) {
	resolve := func(name string) (StaticCallee, bool) {
		if name == "work" {
			return StaticCallee{
				GoName: "static_work",
				Params: []emit.Repr{intRepr()},
				Ret:    intRepr(),
			}, true
		}
		return StaticCallee{}, false
	}
	src := "def run(x: int) -> int:\n    work(x)\n    return x\n"
	got, err := emitWith(t, src, resolve)
	if err != nil {
		t.Fatalf("LowerFuncWith: %v", err)
	}
	for _, want := range []string{
		"_, exc0 := static_work(x)",
		"exc0 != nil",
		"return 0, exc0",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("discarded call is missing %q:\n%s", want, got)
		}
	}
}
