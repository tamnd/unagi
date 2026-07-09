package ir

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/emit"
)

// emitWith lowers a parsed function through the bridge with a resolver in hand
// and prints it, returning the emitted Go source. It is the resolver-aware twin
// of emitOf: a call the resolver knows lowers to a direct Go call, a call it does
// not know keeps the unit boxed.
func emitWith(t *testing.T, src string, resolve CalleeResolver) (string, error) {
	t.Helper()
	fn := parseFunc(t, src)
	f, err := LowerFuncWith(fn, resolve)
	if err != nil {
		return "", err
	}
	out, err := emit.EmitFunc(f)
	if err != nil {
		t.Fatalf("EmitFunc: %v", err)
	}
	return out, nil
}

// floatRepr and intRepr spell the two scalar representations the call tests build
// signatures from, so a resolver can describe a callee without reaching into the
// bridge's private scalarRepr table.
func floatRepr() emit.Repr { return emit.Repr{Go: "float64", Scalar: emit.SFloat, Total: true} }
func intRepr() emit.Repr   { return emit.Repr{Go: "int64", Scalar: emit.SInt} }

// TestLowerStaticCallThreadsError proves the A3 acceptance: a call between two
// static units lowers to a direct Go call on the callee's emitted name, threading
// the D14 error, with no boxing at the boundary. The resolver stands in for the
// call-graph wiring a later slice builds; here it is handed by the test so the
// bridge change is exercised on its own.
func TestLowerStaticCallThreadsError(t *testing.T) {
	resolve := func(name string) (StaticCallee, bool) {
		if name == "scale" {
			return StaticCallee{
				GoName: "static_scale",
				Params: []emit.Repr{floatRepr(), floatRepr()},
				Ret:    floatRepr(),
			}, true
		}
		return StaticCallee{}, false
	}
	src := "def outer(a: float, b: float) -> float:\n    return scale(a, b) + a\n"
	got, err := emitWith(t, src, resolve)
	if err != nil {
		t.Fatalf("LowerFuncWith: %v", err)
	}
	// The call lowers to a direct invocation on the callee's Go name, and the error
	// it returns threads to outer's own return rather than being boxed or dropped.
	for _, want := range []string{
		"func outer(a float64, b float64) (float64, error)",
		"static_scale(a, b)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("static call is missing %q:\n%s", want, got)
		}
	}
	// A threaded error means the emitted body checks the callee's second result and
	// returns it on failure; the zero value it returns on that edge is the float
	// zero, never a boxed object.
	if !strings.Contains(got, "exc0 != nil") {
		t.Errorf("static call does not thread the callee error:\n%s", got)
	}
}

// TestLowerRefusesCallWithoutResolver checks the default: LowerFunc lowers with a
// nil resolver, so any call has no static callee and the unit stays boxed. This is
// the behavior every caller with no known-static callees relies on.
func TestLowerRefusesCallWithoutResolver(t *testing.T) {
	src := "def outer(a: float, b: float) -> float:\n    return scale(a, b) + a\n"
	if _, err := LowerFunc(parseFunc(t, src)); err == nil {
		t.Fatal("a call with no resolver should keep the unit boxed, but it lowered")
	}
}

// TestLowerRefusesUnknownCallee checks a resolver that does not know the callee
// refuses the call the same way a nil resolver does, so a caller only lowers a
// call to a unit the resolver has actually proven static.
func TestLowerRefusesUnknownCallee(t *testing.T) {
	resolve := func(string) (StaticCallee, bool) { return StaticCallee{}, false }
	src := "def outer(a: float, b: float) -> float:\n    return scale(a, b) + a\n"
	if _, err := emitWith(t, src, resolve); err == nil {
		t.Fatal("an unknown callee should keep the unit boxed, but it lowered")
	}
}

// TestLowerRefusesCallArgTypeMismatch checks the bridge refuses a call whose
// argument scalar does not match the callee's parameter representation: handing
// emit a call it cannot type would miscompile, so a mismatch keeps the unit boxed.
func TestLowerRefusesCallArgTypeMismatch(t *testing.T) {
	resolve := func(name string) (StaticCallee, bool) {
		if name == "scale" {
			return StaticCallee{
				GoName: "static_scale",
				Params: []emit.Repr{floatRepr(), floatRepr()},
				Ret:    floatRepr(),
			}, true
		}
		return StaticCallee{}, false
	}
	// n is an int, but scale's first parameter is a float, so the call cannot lower.
	src := "def outer(a: float, n: int) -> float:\n    return scale(n, a)\n"
	if _, err := emitWith(t, src, resolve); err == nil {
		t.Fatal("an argument type mismatch should keep the unit boxed, but it lowered")
	}
}

// TestLowerRefusesCallArgCountMismatch checks the bridge refuses a call whose
// argument count does not match the callee's parameter list, another shape emit
// could not type.
func TestLowerRefusesCallArgCountMismatch(t *testing.T) {
	resolve := func(name string) (StaticCallee, bool) {
		if name == "scale" {
			return StaticCallee{
				GoName: "static_scale",
				Params: []emit.Repr{floatRepr(), floatRepr()},
				Ret:    floatRepr(),
			}, true
		}
		return StaticCallee{}, false
	}
	src := "def outer(a: float) -> float:\n    return scale(a)\n"
	if _, err := emitWith(t, src, resolve); err == nil {
		t.Fatal("an argument count mismatch should keep the unit boxed, but it lowered")
	}
}

// TestLowerRefusesKeywordCallArg checks a keyword argument has no static form: it
// keeps the unit boxed even when the callee is known static.
func TestLowerRefusesKeywordCallArg(t *testing.T) {
	resolve := func(name string) (StaticCallee, bool) {
		if name == "scale" {
			return StaticCallee{
				GoName: "static_scale",
				Params: []emit.Repr{floatRepr(), floatRepr()},
				Ret:    floatRepr(),
			}, true
		}
		return StaticCallee{}, false
	}
	src := "def outer(a: float, b: float) -> float:\n    return scale(a, b=b)\n"
	if _, err := emitWith(t, src, resolve); err == nil {
		t.Fatal("a keyword argument should keep the unit boxed, but it lowered")
	}
}

// TestStaticCallCostCountsOneOp checks the cost model charges a direct call one
// unboxed operation and no guard, so a caller that calls a static unit is scored
// the same as one that inlines a native op, not penalized as if it boxed.
func TestStaticCallCostCountsOneOp(t *testing.T) {
	resolve := func(name string) (StaticCallee, bool) {
		if name == "scale" {
			return StaticCallee{
				GoName: "static_scale",
				Params: []emit.Repr{floatRepr(), floatRepr()},
				Ret:    floatRepr(),
			}, true
		}
		return StaticCallee{}, false
	}
	src := "def outer(a: float, b: float) -> float:\n    return scale(a, b)\n"
	fn := parseFunc(t, src)
	f, err := LowerFuncWith(fn, resolve)
	if err != nil {
		t.Fatalf("LowerFuncWith: %v", err)
	}
	c := CostOf(f)
	if c.UnboxedOps != 1 {
		t.Errorf("a lone static call should be 1 unboxed op, got %d", c.UnboxedOps)
	}
	if c.EntryGuards != 0 || c.LoopGuards != 0 {
		t.Errorf("a static call carries no guard, got entry=%d loop=%d", c.EntryGuards, c.LoopGuards)
	}
}

// TestStaticCallHasNoGuardSite checks the deopt walk opens no site for a call: the
// callee deopts on its own edge, so a caller whose only statement is a call is
// guard-free and its static form may be emitted straight-line.
func TestStaticCallHasNoGuardSite(t *testing.T) {
	resolve := func(name string) (StaticCallee, bool) {
		if name == "scale" {
			return StaticCallee{
				GoName: "static_scale",
				Params: []emit.Repr{floatRepr(), floatRepr()},
				Ret:    floatRepr(),
			}, true
		}
		return StaticCallee{}, false
	}
	src := "def outer(a: float, b: float) -> float:\n    return scale(a, b)\n"
	fn := parseFunc(t, src)
	f, err := LowerFuncWith(fn, resolve)
	if err != nil {
		t.Fatalf("LowerFuncWith: %v", err)
	}
	if sites := GuardSitesOf(f); len(sites) != 0 {
		t.Errorf("a static call carries no guard site, got %d", len(sites))
	}
}

// TestStaticCallGuardedArgOpensSite checks the seam the other way: a call whose
// argument carries an int overflow guard opens exactly one site, since the guard
// lives in the caller's own argument expression, not the callee.
func TestStaticCallGuardedArgOpensSite(t *testing.T) {
	resolve := func(name string) (StaticCallee, bool) {
		if name == "twice" {
			return StaticCallee{
				GoName: "static_twice",
				Params: []emit.Repr{intRepr()},
				Ret:    intRepr(),
			}, true
		}
		return StaticCallee{}, false
	}
	src := "def outer(m: int, n: int) -> int:\n    return twice(m + n)\n"
	fn := parseFunc(t, src)
	f, err := LowerFuncWith(fn, resolve)
	if err != nil {
		t.Fatalf("LowerFuncWith: %v", err)
	}
	if sites := GuardSitesOf(f); len(sites) != 1 {
		t.Errorf("a guarded call argument opens one site, got %d", len(sites))
	}
}
