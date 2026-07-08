package ir

import (
	"testing"

	"github.com/tamnd/unagi/pkg/emit"
)

// costOfSrc parses and lowers a function, then measures it, failing on any error
// so a test reads as one call.
func costOfSrc(t *testing.T, src string) Cost {
	t.Helper()
	f, err := LowerFunc(parseFunc(t, src))
	if err != nil {
		t.Fatalf("LowerFunc: %v", err)
	}
	return CostOf(f)
}

func TestCostCountsFloatOpsWithoutGuards(t *testing.T) {
	// Two total float operations, no overflow guards.
	c := costOfSrc(t, "def f(a: float, b: float, c: float) -> float:\n    return a * b + c\n")
	if c.UnboxedOps != 2 {
		t.Errorf("UnboxedOps = %d, want 2", c.UnboxedOps)
	}
	if c.EntryGuards != 0 || c.LoopGuards != 0 {
		t.Errorf("float arithmetic should carry no guards, got entry=%d loop=%d", c.EntryGuards, c.LoopGuards)
	}
}

func TestCostGuardsEveryIntOperation(t *testing.T) {
	// Three int adds, each an overflow-guarded operation.
	c := costOfSrc(t, "def f(a: int, b: int, c: int, d: int) -> int:\n    return a + b + c + d\n")
	if c.UnboxedOps != 3 {
		t.Errorf("UnboxedOps = %d, want 3", c.UnboxedOps)
	}
	if c.EntryGuards != 3 {
		t.Errorf("each int add should carry a guard, got %d", c.EntryGuards)
	}
}

func TestCostTrueDivisionIsUnguardedFloat(t *testing.T) {
	// True division always yields a float, so it is a total operation with no
	// overflow guard even though its operands are ints.
	c := costOfSrc(t, "def f(a: int, b: int) -> float:\n    return a / b\n")
	if c.UnboxedOps != 1 {
		t.Errorf("UnboxedOps = %d, want 1", c.UnboxedOps)
	}
	if c.EntryGuards != 0 {
		t.Errorf("division should not carry an overflow guard, got %d", c.EntryGuards)
	}
}

func TestCostCountsAugAssign(t *testing.T) {
	// The seed accumulator binds an int local then adds into it twice; the two
	// += operations are guarded int adds.
	c := costOfSrc(t, "def f(a: int, b: int) -> int:\n    s = 0\n    s += a\n    s += b\n    return s\n")
	if c.UnboxedOps != 2 {
		t.Errorf("UnboxedOps = %d, want 2", c.UnboxedOps)
	}
	if c.EntryGuards != 2 {
		t.Errorf("each int += should carry a guard, got %d", c.EntryGuards)
	}
}

// TestCostIgnoresUnknownNodes guards the walk against a node the bridge never
// builds: a bare variable return contributes no operations.
func TestCostIgnoresUnknownNodes(t *testing.T) {
	f := emit.Func{
		Name:   "id",
		Params: []emit.Param{{Name: "a", Repr: emit.Repr{Go: "float64", Scalar: emit.SFloat, Total: true}}},
		Ret:    emit.Repr{Go: "float64", Scalar: emit.SFloat, Total: true},
		Body:   []emit.Stmt{emit.Return{Value: emit.Var{Name: "a", Repr: emit.Repr{Go: "float64", Scalar: emit.SFloat, Total: true}}}},
	}
	if c := CostOf(f); c.UnboxedOps != 0 || c.EntryGuards != 0 {
		t.Errorf("a bare return should have no cost, got %+v", c)
	}
}
