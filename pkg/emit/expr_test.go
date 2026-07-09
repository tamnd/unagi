package emit

import (
	"strings"
	"testing"
)

// emitOneReturn wraps a single expression in a returning function and emits it,
// so a test can assert on the lowered expression in context.
func emitOneReturn(t *testing.T, name string, ret Repr, params []Param, e Expr) string {
	t.Helper()
	got, err := EmitFunc(Func{Name: name, Params: params, Ret: ret, Body: []Stmt{Return{Value: e}}})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestMixedIntFloatCoerces(t *testing.T) {
	fR, iR, _ := reprs()
	// n * 2.0 with n an int: the int side coerces to float64 and the op is a bare,
	// unguarded float multiply.
	src := emitOneReturn(t, "scale", fR, []Param{{Name: "n", Repr: iR}},
		Bin{Op: OpMul, L: Var{Name: "n", Repr: iR}, R: Float{V: 2}})
	if !strings.Contains(src, "return float64(n) * 2.0, nil") {
		t.Fatalf("mixed multiply should coerce the int side to float64:\n%s", src)
	}
	if strings.Contains(src, "rt.MulInt64") {
		t.Fatalf("a float result must not emit an overflow guard:\n%s", src)
	}
}

func TestTrueDivisionGuardsZero(t *testing.T) {
	fR, iR, _ := reprs()
	// a / b on two ints is float division in Python: both coerce to float64, the
	// divisor is zero-checked, and the result is a float.
	src := emitOneReturn(t, "ratio", fR, []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}},
		Bin{Op: OpDiv, L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: iR}})
	if !strings.Contains(src, "if float64(b) == 0") {
		t.Fatalf("division should guard a zero divisor:\n%s", src)
	}
	if !strings.Contains(src, `rt.ZeroDivisionError("division by zero")`) {
		t.Fatalf("the zero guard should raise ZeroDivisionError:\n%s", src)
	}
	if !strings.Contains(src, "return float64(a) / float64(b), nil") {
		t.Fatalf("division should coerce both sides and divide:\n%s", src)
	}
}

func TestFloorDivGuardsZeroAndOverflow(t *testing.T) {
	_, iR, _ := reprs()
	// a // b on two ints is int floor division: the divisor is zero-checked with the
	// bare "division by zero" python3.14 raises for every zero divisor, the value comes
	// through the runtime helper that floors toward negative infinity, and the one
	// overflow (MinInt64 // -1) routes to the unit's deopt edge like any other int
	// overflow.
	src := emitOneReturn(t, "quot", iR, []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}},
		Bin{Op: OpFloorDiv, L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: iR}})
	if !strings.Contains(src, "if b == 0") {
		t.Fatalf("floor division should guard a zero divisor:\n%s", src)
	}
	if !strings.Contains(src, `rt.ZeroDivisionError("division by zero")`) {
		t.Fatalf("the int zero guard should raise the bare division-by-zero message:\n%s", src)
	}
	if !strings.Contains(src, "rt.FloorDivInt64(a, b)") {
		t.Fatalf("floor division should route through the flooring helper:\n%s", src)
	}
	if !strings.Contains(src, "quot_deopt0(a, b)") {
		t.Fatalf("the overflow flag should route to the deopt edge:\n%s", src)
	}
	if strings.Contains(src, "a / b") {
		t.Fatalf("floor division must not lower to a bare Go divide that truncates:\n%s", src)
	}
}

func TestIndexGuardsBoundsAndDeopts(t *testing.T) {
	_, iR, _ := reprs()
	elem := iR
	intList := Repr{Go: "[]int64", Total: true, Elem: &elem}
	// xs[i] on a list[int] and an int index: the base and the index bind to temps
	// in evaluation order, the bounds guard `i < 0 || i >= int64(len(xs))` fails to
	// the unit's deopt edge so a negative or out-of-range index leaves for the boxed
	// twin, and the in-range read is a plain Go slice access yielding the element
	// representation. No boxing crosses the read.
	src := emitOneReturn(t, "get", iR,
		[]Param{{Name: "xs", Repr: intList}, {Name: "i", Repr: iR}},
		Index{Base: Var{Name: "xs", Repr: intList}, Idx: Var{Name: "i", Repr: iR}})
	if !strings.Contains(src, "t0 := xs") || !strings.Contains(src, "t1 := i") {
		t.Fatalf("the base and index should bind to temps in order:\n%s", src)
	}
	if !strings.Contains(src, "if t1 < 0 || t1 >= int64(len(t0)) {") {
		t.Fatalf("the read should guard the index against the slice bounds:\n%s", src)
	}
	if !strings.Contains(src, "return get_deopt0(xs, i)") {
		t.Fatalf("an out-of-range index should deopt to the boxed twin:\n%s", src)
	}
	if !strings.Contains(src, "return t0[t1], nil") {
		t.Fatalf("the in-range read should be a plain slice access:\n%s", src)
	}
	if strings.Contains(src, "objects.") {
		t.Fatalf("the read must not box the element:\n%s", src)
	}
}

func TestListLiteralLowersToSliceLiteral(t *testing.T) {
	_, iR, _ := reprs()
	intList := Repr{Go: "[]int64", Total: true, Elem: &iR}
	// [10, 20, 30] as a list[int] -> the Go slice literal []int64{10, 20, 30}, a
	// pure value with no guard.
	src := emitOneReturn(t, "nums", intList, nil,
		ListLit{Elem: iR, Items: []Expr{Int{V: 10}, Int{V: 20}, Int{V: 30}}})
	if !strings.Contains(src, "return []int64{10, 20, 30}, nil") {
		t.Fatalf("a list literal should lower to a Go slice literal:\n%s", src)
	}
}

func TestListLiteralCoercesIntItemsToFloat(t *testing.T) {
	fR, iR, _ := reprs()
	floatList := Repr{Go: "[]float64", Total: true, Elem: &fR}
	// A float list with an int item coerces the item up, the same coercion a mixed
	// scalar assignment uses, so the slice stays uniform float64.
	src := emitOneReturn(t, "nums", floatList, nil,
		ListLit{Elem: fR, Items: []Expr{Float{V: 1.5}, Int{V: 2}}})
	if !strings.Contains(src, "[]float64{1.5, float64(2)}") {
		t.Fatalf("an int item in a float list should coerce up:\n%s", src)
	}
	_ = iR
}

func TestListLiteralRejectsMixedScalarClass(t *testing.T) {
	_, iR, _ := reprs()
	intList := Repr{Go: "[]int64", Total: true, Elem: &iR}
	// A str item in an int list is a heterogeneous literal inference should never
	// have proven to a uniform element type; emit refuses it rather than building a
	// slice of the wrong element type.
	_, err := EmitFunc(Func{
		Name: "bad", Ret: intList,
		Body: []Stmt{Return{Value: ListLit{Elem: iR, Items: []Expr{Int{V: 1}, Str{V: "x"}}}}},
	})
	if err == nil {
		t.Fatal("a mixed-scalar-class list literal should be refused, not miscompiled")
	}
}

func TestIndexRejectsNonListBase(t *testing.T) {
	_, iR, _ := reprs()
	// Indexing a scalar is an inference bug reaching emit; the lowering refuses it
	// rather than emitting a Go slice access on a non-slice, which would not compile.
	_, err := EmitFunc(Func{
		Name: "bad", Ret: iR,
		Body: []Stmt{Return{Value: Index{Base: Var{Name: "n", Repr: iR}, Idx: Int{V: 0}}}},
	})
	if err == nil {
		t.Fatal("indexing a non-list base should be refused, not miscompiled")
	}
}

func TestFloorDivOnFloatIsRefused(t *testing.T) {
	fR, iR, _ := reprs()
	// A float operand keeps floor division boxed at M4, so the static tier refuses it
	// rather than lowering a float // that would need the runtime's flooring math.
	_, err := EmitFunc(Func{
		Name: "bad", Ret: fR,
		Body: []Stmt{Return{Value: Bin{Op: OpFloorDiv, L: Var{Name: "a", Repr: iR}, R: Float{V: 2}}}},
	})
	if err == nil {
		t.Fatal("floor division with a float operand should be refused, not miscompiled")
	}
}

func TestModuloGuardsZeroWithoutDeopt(t *testing.T) {
	_, iR, _ := reprs()
	// a % b on two ints is the floored modulo: the divisor is zero-checked with the
	// bare "division by zero" message, the value comes through the runtime helper inline,
	// and there is no overflow flag or deopt edge because a floored remainder never
	// overflows int64.
	src := emitOneReturn(t, "rem", iR, []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}},
		Bin{Op: OpMod, L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: iR}})
	if !strings.Contains(src, "if b == 0") {
		t.Fatalf("modulo should guard a zero divisor:\n%s", src)
	}
	if !strings.Contains(src, `rt.ZeroDivisionError("division by zero")`) {
		t.Fatalf("the int zero guard should raise the bare division-by-zero message:\n%s", src)
	}
	if !strings.Contains(src, "return rt.FloorModInt64(a, b), nil") {
		t.Fatalf("modulo should return the flooring helper inline:\n%s", src)
	}
	if strings.Contains(src, "deopt") || strings.Contains(src, "ovf") {
		t.Fatalf("modulo never overflows, so it must carry no deopt edge:\n%s", src)
	}
	if strings.Contains(src, "a % b") {
		t.Fatalf("modulo must not lower to a bare Go %% that keeps the dividend's sign:\n%s", src)
	}
}

func TestModuloOnFloatIsRefused(t *testing.T) {
	fR, iR, _ := reprs()
	// A float operand keeps modulo boxed at M4, so the static tier refuses it.
	_, err := EmitFunc(Func{
		Name: "bad", Ret: fR,
		Body: []Stmt{Return{Value: Bin{Op: OpMod, L: Var{Name: "a", Repr: iR}, R: Float{V: 2}}}},
	})
	if err == nil {
		t.Fatal("modulo with a float operand should be refused, not miscompiled")
	}
}

func TestPowerGuardsNegativeAndOverflow(t *testing.T) {
	_, iR, _ := reprs()
	// a ** b on two ints is the int power: the runtime helper folds both escape
	// hatches into one deopt flag, so a single edge routes a negative exponent (which
	// Python turns into a float, and 0 ** -1 raises) and an int64 overflow (which
	// Python spills to a big int) to the boxed twin. There is no zero-divisor check
	// because ** has no zero divisor.
	src := emitOneReturn(t, "power", iR, []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}},
		Bin{Op: OpPow, L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: iR}})
	if !strings.Contains(src, "rt.PowInt64(a, b)") {
		t.Fatalf("power should route through the repeated-squaring helper:\n%s", src)
	}
	if !strings.Contains(src, "power_deopt0(a, b)") {
		t.Fatalf("the deopt flag should route to the deopt edge:\n%s", src)
	}
	if strings.Contains(src, "ZeroDivisionError") {
		t.Fatalf("power has no zero divisor, so it must carry no zero-division guard:\n%s", src)
	}
	if strings.Contains(src, "a ** b") {
		t.Fatalf("Go has no ** operator, so power must not lower to a bare token:\n%s", src)
	}
}

func TestPowerOnFloatIsRefused(t *testing.T) {
	fR, iR, _ := reprs()
	// A float operand keeps power boxed at M4, so the static tier refuses it rather
	// than lowering a float ** that would need math.Pow.
	_, err := EmitFunc(Func{
		Name: "bad", Ret: fR,
		Body: []Stmt{Return{Value: Bin{Op: OpPow, L: Var{Name: "a", Repr: iR}, R: Float{V: 2}}}},
	})
	if err == nil {
		t.Fatal("power with a float operand should be refused, not miscompiled")
	}
}

func TestBitwiseOpsAreTotalInt(t *testing.T) {
	_, iR, _ := reprs()
	// a & b, a | b, a ^ b on two ints lower to Go's native operator with an int
	// result and no guard: a two's-complement bit op on int64 matches Python's
	// infinite-precision answer for any operands that fit int64.
	cases := []struct {
		op   Op
		want string
	}{
		{OpBitAnd, "return a & b, nil"},
		{OpBitOr, "return a | b, nil"},
		{OpBitXor, "return a ^ b, nil"},
	}
	for _, tc := range cases {
		src := emitOneReturn(t, "bits", iR, []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}},
			Bin{Op: tc.op, L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: iR}})
		if !strings.Contains(src, tc.want) {
			t.Fatalf("%s should lower to the native operator %q:\n%s", tc.op, tc.want, src)
		}
		if strings.Contains(src, "deopt") || strings.Contains(src, "ovf") || strings.Contains(src, "rt.") {
			t.Fatalf("%s is total, so it must carry no guard or runtime helper:\n%s", tc.op, src)
		}
	}
}

func TestBitwiseOnFloatIsRefused(t *testing.T) {
	fR, iR, _ := reprs()
	// A float operand is a TypeError for bitwise ops in Python, so the static tier
	// refuses it rather than lowering a Go bit op that would not compile on a float.
	for _, op := range []Op{OpBitAnd, OpBitOr, OpBitXor} {
		_, err := EmitFunc(Func{
			Name: "bad", Ret: fR,
			Body: []Stmt{Return{Value: Bin{Op: op, L: Var{Name: "a", Repr: iR}, R: Float{V: 2}}}},
		})
		if err == nil {
			t.Fatalf("%s with a float operand should be refused, not miscompiled", op)
		}
	}
}

func TestLeftShiftGuardsNegativeAndOverflow(t *testing.T) {
	_, iR, _ := reprs()
	// a << b on two ints is int left shift: a negative count raises ValueError, the
	// value comes through the runtime helper, and the overflow past int64 routes to the
	// deopt edge like any other int overflow.
	src := emitOneReturn(t, "shl", iR, []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}},
		Bin{Op: OpLShift, L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: iR}})
	if !strings.Contains(src, "if b < 0") {
		t.Fatalf("left shift should guard a negative count:\n%s", src)
	}
	if !strings.Contains(src, `rt.ValueError("negative shift count")`) {
		t.Fatalf("the negative-count guard should raise ValueError:\n%s", src)
	}
	if !strings.Contains(src, "rt.LShiftInt64(a, b)") {
		t.Fatalf("left shift should route through the overflow-checking helper:\n%s", src)
	}
	if !strings.Contains(src, "shl_deopt0(a, b)") {
		t.Fatalf("the overflow flag should route to the deopt edge:\n%s", src)
	}
	if strings.Contains(src, "a << b") {
		t.Fatalf("left shift must not lower to a bare Go shift that wraps silently:\n%s", src)
	}
}

func TestRightShiftGuardsNegativeWithoutDeopt(t *testing.T) {
	_, iR, _ := reprs()
	// a >> b on two ints is arithmetic right shift: a negative count raises ValueError,
	// the value comes through the runtime helper inline, and there is no overflow flag
	// or deopt edge because an arithmetic right shift only shrinks the magnitude.
	src := emitOneReturn(t, "shr", iR, []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}},
		Bin{Op: OpRShift, L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: iR}})
	if !strings.Contains(src, "if b < 0") {
		t.Fatalf("right shift should guard a negative count:\n%s", src)
	}
	if !strings.Contains(src, `rt.ValueError("negative shift count")`) {
		t.Fatalf("the negative-count guard should raise ValueError:\n%s", src)
	}
	if !strings.Contains(src, "return rt.RShiftInt64(a, b), nil") {
		t.Fatalf("right shift should return the helper inline:\n%s", src)
	}
	if strings.Contains(src, "deopt") || strings.Contains(src, "ovf") {
		t.Fatalf("right shift never overflows, so it must carry no deopt edge:\n%s", src)
	}
}

func TestShiftOnFloatIsRefused(t *testing.T) {
	fR, iR, _ := reprs()
	// A float operand is a TypeError for shifts in Python, so the static tier refuses
	// it rather than lowering a form that would not compile on a float.
	for _, op := range []Op{OpLShift, OpRShift} {
		_, err := EmitFunc(Func{
			Name: "bad", Ret: fR,
			Body: []Stmt{Return{Value: Bin{Op: op, L: Var{Name: "a", Repr: iR}, R: Float{V: 2}}}},
		})
		if err == nil {
			t.Fatalf("%s with a float operand should be refused, not miscompiled", op)
		}
	}
}

func TestIntAugAssignIsGuarded(t *testing.T) {
	_, iR, _ := reprs()
	// An int accumulator lowers through the guarded add, not a bare +=, so it
	// cannot wrap silently.
	got, err := EmitFunc(Func{
		Name:   "acc",
		Params: []Param{{Name: "n", Repr: iR}},
		Ret:    iR,
		Body: []Stmt{
			Define{Name: "s", Value: Int{V: 0}},
			AugAssign{Name: "s", Repr: iR, Value: Var{Name: "n", Repr: iR}},
			Return{Value: Var{Name: "s", Repr: iR}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "rt.AddInt64(s, n)") {
		t.Fatalf("int accumulation should route through the overflow-checked add:\n%s", got)
	}
	if strings.Contains(got, "s += n") {
		t.Fatalf("int accumulation must not use a bare += that can wrap:\n%s", got)
	}
}

func TestIntSubMulAugAssignAreGuarded(t *testing.T) {
	_, iR, _ := reprs()
	// `-=` and `*=` on an int accumulator route through the same overflow-checked
	// helpers `+=` does, never a bare compound assignment that could wrap.
	cases := []struct {
		op    Op
		want  string
		wrong string
	}{
		{OpSub, "rt.SubInt64(s, n)", "s -= n"},
		{OpMul, "rt.MulInt64(s, n)", "s *= n"},
	}
	for _, tc := range cases {
		got, err := EmitFunc(Func{
			Name:   "acc",
			Params: []Param{{Name: "n", Repr: iR}},
			Ret:    iR,
			Body: []Stmt{
				Define{Name: "s", Value: Int{V: 0}},
				AugAssign{Name: "s", Op: tc.op, Repr: iR, Value: Var{Name: "n", Repr: iR}},
				Return{Value: Var{Name: "s", Repr: iR}},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, tc.want) {
			t.Fatalf("int %s= should route through %q:\n%s", tc.op, tc.want, got)
		}
		if strings.Contains(got, tc.wrong) {
			t.Fatalf("int %s= must not use a bare %q that can wrap:\n%s", tc.op, tc.wrong, got)
		}
	}
}

func TestFloatAugAssignUsesCompoundToken(t *testing.T) {
	fR, _, _ := reprs()
	// Float arithmetic is total, so `-=` and `*=` lower to Go's compound assignment
	// directly with no overflow guard.
	cases := []struct {
		op   Op
		want string
	}{
		{OpSub, "s -= x"},
		{OpMul, "s *= x"},
	}
	for _, tc := range cases {
		got, err := EmitFunc(Func{
			Name:   "acc",
			Params: []Param{{Name: "x", Repr: fR}},
			Ret:    fR,
			Body: []Stmt{
				Define{Name: "s", Value: Float{V: 1.0}},
				AugAssign{Name: "s", Op: tc.op, Repr: fR, Value: Var{Name: "x", Repr: fR}},
				Return{Value: Var{Name: "s", Repr: fR}},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, tc.want) {
			t.Fatalf("float %s= should lower to %q:\n%s", tc.op, tc.want, got)
		}
	}
}

func TestNonNumericOperandRejected(t *testing.T) {
	fR, _, _ := reprs()
	strR := Repr{Go: "string", Scalar: SStr, Total: true}
	_, err := EmitFunc(Func{
		Name: "bad", Ret: fR,
		Body: []Stmt{Return{Value: Bin{Op: OpAdd, L: Var{Name: "s", Repr: strR}, R: Float{V: 1}}}},
	})
	if err == nil {
		t.Fatal("arithmetic on a string operand should be refused, not miscompiled")
	}
}

func TestRangeNeedsList(t *testing.T) {
	fR, iR, _ := reprs()
	_, err := EmitFunc(Func{
		Name: "bad", Ret: fR,
		Body: []Stmt{ForRange{Bind: "x", Over: Var{Name: "n", Repr: iR}, Body: nil}},
	})
	if err == nil {
		t.Fatal("ranging a non-list operand should be refused")
	}
}

func TestOpStrings(t *testing.T) {
	for op, want := range map[Op]string{OpAdd: "+", OpSub: "-", OpMul: "*", OpDiv: "/", OpFloorDiv: "//", OpMod: "%", OpPow: "**", OpBitAnd: "&", OpBitOr: "|", OpBitXor: "^", OpLShift: "<<", OpRShift: ">>"} {
		if op.String() != want {
			t.Fatalf("Op(%d).String() = %q, want %q", op, op.String(), want)
		}
	}
}

// TestDeterministic emits the same function twice and requires byte-identical
// output, the property the partition determinism story rests on downstream.
func TestDeterministic(t *testing.T) {
	_, iR, _ := reprs()
	f := Func{
		Name:   "poly",
		Params: []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}},
		Ret:    iR,
		Body: []Stmt{Return{Value: Bin{Op: OpAdd,
			L: Bin{Op: OpMul, L: Var{Name: "a", Repr: iR}, R: Var{Name: "a", Repr: iR}},
			R: Var{Name: "b", Repr: iR}}}},
	}
	a, _ := EmitFunc(f)
	b, _ := EmitFunc(f)
	if a != b {
		t.Fatal("emit should be deterministic across builds")
	}
}
