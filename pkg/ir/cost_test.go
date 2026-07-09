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

func TestCostFloorDivisionIsGuardedInt(t *testing.T) {
	// Floor division yields an int and can overflow at MinInt64 // -1, so it carries
	// an overflow guard and a deopt edge, the same as an int add.
	c := costOfSrc(t, "def f(a: int, b: int) -> int:\n    return a // b\n")
	if c.UnboxedOps != 1 {
		t.Errorf("UnboxedOps = %d, want 1", c.UnboxedOps)
	}
	if c.EntryGuards != 1 {
		t.Errorf("floor division should carry an overflow guard, got %d", c.EntryGuards)
	}
}

func TestCostModuloIsUnguardedInt(t *testing.T) {
	// Modulo yields an int but its floored remainder is always smaller than the
	// divisor, so it cannot overflow: it is a total operation with only a zero-divisor
	// semantic guard, no overflow guard and no deopt edge. If the census counted it as
	// guarded, a modulo function would auto-box on the guard budget for a guard that
	// never fires, and the deopt-site walk would open a phantom site.
	c := costOfSrc(t, "def f(a: int, b: int) -> int:\n    return a % b\n")
	if c.UnboxedOps != 1 {
		t.Errorf("UnboxedOps = %d, want 1", c.UnboxedOps)
	}
	if c.EntryGuards != 0 {
		t.Errorf("modulo should not carry an overflow guard, got %d", c.EntryGuards)
	}
}

func TestCostPowerIsGuardedInt(t *testing.T) {
	// Power yields an int but deopts on a negative exponent (Python turns it into a
	// float) and on an int64 overflow, so it carries a guard and a deopt edge, the same
	// as floor division. The census must count it as guarded so the partitioner builds
	// its boxed twin and resume plan.
	c := costOfSrc(t, "def f(a: int, b: int) -> int:\n    return a ** b\n")
	if c.UnboxedOps != 1 {
		t.Errorf("UnboxedOps = %d, want 1", c.UnboxedOps)
	}
	if c.EntryGuards != 1 {
		t.Errorf("power should carry a deopt guard, got %d", c.EntryGuards)
	}
}

func TestCostBitwiseIsUnguardedInt(t *testing.T) {
	// The logical bitwise ops yield an int but never overflow: a two's-complement bit
	// op on int64 stays in int64, so they are total with no guard and no deopt edge,
	// the same classification as modulo. If the census counted one as guarded, a
	// bitwise function would auto-box on the guard budget for a guard that never fires.
	for _, src := range []string{
		"def f(a: int, b: int) -> int:\n    return a & b\n",
		"def f(a: int, b: int) -> int:\n    return a | b\n",
		"def f(a: int, b: int) -> int:\n    return a ^ b\n",
	} {
		c := costOfSrc(t, src)
		if c.UnboxedOps != 1 {
			t.Errorf("UnboxedOps = %d, want 1 for %q", c.UnboxedOps, src)
		}
		if c.EntryGuards != 0 {
			t.Errorf("bitwise op should not carry a guard, got %d for %q", c.EntryGuards, src)
		}
	}
}

func TestCostLeftShiftIsGuardedInt(t *testing.T) {
	// Left shift yields an int and can overflow past int64, so it carries an overflow
	// guard and a deopt edge, the same as an int add. The negative-count check is a
	// separate semantic ValueError, not counted here.
	c := costOfSrc(t, "def f(a: int, b: int) -> int:\n    return a << b\n")
	if c.UnboxedOps != 1 {
		t.Errorf("UnboxedOps = %d, want 1", c.UnboxedOps)
	}
	if c.EntryGuards != 1 {
		t.Errorf("left shift should carry an overflow guard, got %d", c.EntryGuards)
	}
}

func TestCostRightShiftIsUnguardedInt(t *testing.T) {
	// Right shift yields an int but an arithmetic shift only shrinks the magnitude, so
	// it never overflows: it is a total operation with only the negative-count semantic
	// guard, no overflow guard and no deopt edge, the same classification as modulo.
	c := costOfSrc(t, "def f(a: int, b: int) -> int:\n    return a >> b\n")
	if c.UnboxedOps != 1 {
		t.Errorf("UnboxedOps = %d, want 1", c.UnboxedOps)
	}
	if c.EntryGuards != 0 {
		t.Errorf("right shift should not carry an overflow guard, got %d", c.EntryGuards)
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

func TestCostCountsComparisonAndConnective(t *testing.T) {
	// A comparison and a connective each count one unboxed operation, and neither
	// carries an overflow guard: reading two values to compare produces no new int.
	c := costOfSrc(t, "def f(a: int, b: int, c: int) -> bool:\n    return a < b and b < c\n")
	if c.UnboxedOps != 3 {
		t.Errorf("two compares and one and should be three ops, got %d", c.UnboxedOps)
	}
	if c.EntryGuards != 0 || c.LoopGuards != 0 {
		t.Errorf("a comparison and a connective carry no guards, got entry=%d loop=%d", c.EntryGuards, c.LoopGuards)
	}
}

func TestCostGuardsIntOpNestedInComparison(t *testing.T) {
	// The load-bearing case: an int add hidden inside a comparison still emits its
	// overflow guard and a deopt edge, so the census must count it. Missing it would
	// mark a function that actually deopts as guard-free static, which D4 forbids.
	c := costOfSrc(t, "def f(a: int, b: int, c: int) -> bool:\n    return a + b < c\n")
	if c.EntryGuards != 1 {
		t.Errorf("the int add inside the comparison should contribute one guard, got %d", c.EntryGuards)
	}
}

func TestCostGuardsIntOpNestedInConnectiveAndNot(t *testing.T) {
	// The same guard must be seen through a connective and through not: `not (a + b
	// < c)` wraps the guarded add under a Not, and the add under an And operand.
	if c := costOfSrc(t, "def f(a: int, b: int, c: int) -> bool:\n    return not a + b < c\n"); c.EntryGuards != 1 {
		t.Errorf("the int add under not should contribute one guard, got %d", c.EntryGuards)
	}
	if c := costOfSrc(t, "def f(a: int, b: int, c: int, d: int) -> bool:\n    return a + b < c and c < d\n"); c.EntryGuards != 1 {
		t.Errorf("the int add under an and operand should contribute one guard, got %d", c.EntryGuards)
	}
}

func TestCostGuardsIntOpInIfConditionAndArm(t *testing.T) {
	// A guarded int add in an if condition contributes its overflow guard: the walk
	// must descend into the condition, or a function that deopts on the condition
	// would read as guard-free static, the mislabel D4 forbids.
	if c := costOfSrc(t, "def f(a: int, b: int) -> int:\n    if a + b:\n        return 1\n    return 0\n"); c.EntryGuards != 1 {
		t.Errorf("the int add in the if condition should contribute one guard, got %d", c.EntryGuards)
	}
	// A guarded int add inside a branch arm contributes its guard the same way, so
	// the walk descends into both arms.
	src := "def f(a: int, b: int) -> int:\n    if a > b:\n        return a + b\n    else:\n        return b\n"
	if c := costOfSrc(t, src); c.EntryGuards != 1 {
		t.Errorf("the int add inside the then arm should contribute one guard, got %d", c.EntryGuards)
	}
}

func TestCostGuardFreeIfIsStatic(t *testing.T) {
	// An if on a scalar condition with guard-free arms carries no guard, so it stays
	// in the guard-free static set the differential runner and partitioner adopt.
	c := costOfSrc(t, "def f(n: int) -> int:\n    if n:\n        return 1\n    return 0\n")
	if c.EntryGuards != 0 || c.LoopGuards != 0 {
		t.Errorf("a guard-free if should carry no guards, got entry=%d loop=%d", c.EntryGuards, c.LoopGuards)
	}
}

// TestCostBranchJoinIsGuardFree proves the join declaration adds nothing to the
// census: a name both arms bind to a literal hoists to a `var x int64` and two
// assignments, none of which is arithmetic, so the function stays guard-free static.
func TestCostBranchJoinIsGuardFree(t *testing.T) {
	src := "def f(c: int) -> int:\n    if c > 0:\n        x = 10\n    else:\n        x = 20\n    return x\n"
	c := costOfSrc(t, src)
	if c.EntryGuards != 0 || c.LoopGuards != 0 {
		t.Errorf("a literal branch join should carry no guards, got entry=%d loop=%d", c.EntryGuards, c.LoopGuards)
	}
	// The one comparison in the condition is the only counted operation; the join
	// declaration and the two assignments carry no arithmetic of their own.
	if c.UnboxedOps != 1 {
		t.Errorf("only the condition comparison should count, got %d ops", c.UnboxedOps)
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
