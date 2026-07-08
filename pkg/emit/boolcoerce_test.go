package emit

import (
	"strings"
	"testing"
)

// This file covers bool operands reaching arithmetic (03_arithmetic_float.md
// lines 25 and 31, cross-referenced from 02). Python's bool is a subtype of int,
// so a bool in a numeric expression counts as 1 or 0: `True + 1.0` is `2.0`,
// `True + 1` is `2`, `True + True` is `2`, and a comparison result feeding
// arithmetic (`(a < b) + c`) coerces the same way. The static tier makes the
// coercion explicit through rt.BoolToInt, since Go has no implicit bool-to-number
// conversion, and asserts the emitted coercion rather than a raw bool-in-arithmetic
// type error.

func TestBoolPlusFloatCoercesToFloat(t *testing.T) {
	fR, _, _ := reprs()
	// b + 1.0 with b a bool: the bool promotes through int to float64 and the op is
	// a bare, unguarded float add. `True + 1.0 == 2.0` in CPython.
	src := emitOneReturn(t, "f", fR, []Param{{Name: "b", Repr: boolRepr()}},
		Bin{Op: OpAdd, L: Var{Name: "b", Repr: boolRepr()}, R: Float{V: 1}})
	if !strings.Contains(src, "return float64(rt.BoolToInt(b)) + 1.0, nil") {
		t.Fatalf("a bool meeting a float should coerce through int to float64:\n%s", src)
	}
	if strings.Contains(src, "rt.AddInt64") {
		t.Fatalf("a float result must not emit an overflow guard:\n%s", src)
	}
}

func TestBoolPlusIntIsGuardedInt(t *testing.T) {
	_, iR, _ := reprs()
	// b + n with b a bool and n an int: bool is a subtype of int, so the result is a
	// plain int and rides the guarded add. `True + 1 == 2`.
	src := emitOneReturn(t, "f", iR, []Param{{Name: "b", Repr: boolRepr()}, {Name: "n", Repr: iR}},
		Bin{Op: OpAdd, L: Var{Name: "b", Repr: boolRepr()}, R: Var{Name: "n", Repr: iR}})
	if !strings.Contains(src, "rt.AddInt64(rt.BoolToInt(b), n)") {
		t.Fatalf("a bool meeting an int should coerce to int64 and ride the guarded add:\n%s", src)
	}
}

func TestBoolTimesBoolIsInt(t *testing.T) {
	_, iR, _ := reprs()
	// Two bools multiply as ints: `True * True == 1`, an int, so both sides coerce to
	// int64 through the guarded multiply.
	src := emitOneReturn(t, "f", iR, []Param{{Name: "a", Repr: boolRepr()}, {Name: "b", Repr: boolRepr()}},
		Bin{Op: OpMul, L: Var{Name: "a", Repr: boolRepr()}, R: Var{Name: "b", Repr: boolRepr()}})
	if !strings.Contains(src, "rt.MulInt64(rt.BoolToInt(a), rt.BoolToInt(b))") {
		t.Fatalf("two bools should both coerce to int64:\n%s", src)
	}
}

func TestCompareResultFeedingArithmetic(t *testing.T) {
	_, iR, _ := reprs()
	// (a < b) + c: a comparison yields a bool, which coerces to int64 when it feeds a
	// numeric add, rather than raising a bool-in-arithmetic type error. `(a < b) + c`.
	src := emitOneReturn(t, "f", iR, []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}, {Name: "c", Repr: iR}},
		Bin{Op: OpAdd, L: Cmp{Op: CmpLt, L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: iR}}, R: Var{Name: "c", Repr: iR}})
	if !strings.Contains(src, "rt.BoolToInt(a < b)") {
		t.Fatalf("a comparison feeding arithmetic should coerce the bool to int64:\n%s", src)
	}
	if !strings.Contains(src, "rt.AddInt64(rt.BoolToInt(a < b), c)") {
		t.Fatalf("the coerced comparison should ride the guarded add:\n%s", src)
	}
}
