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
	// integer-specific message, the value comes through the runtime helper that floors
	// toward negative infinity, and the one overflow (MinInt64 // -1) routes to the
	// unit's deopt edge like any other int overflow.
	src := emitOneReturn(t, "quot", iR, []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}},
		Bin{Op: OpFloorDiv, L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: iR}})
	if !strings.Contains(src, "if b == 0") {
		t.Fatalf("floor division should guard a zero divisor:\n%s", src)
	}
	if !strings.Contains(src, `rt.ZeroDivisionError("integer division or modulo by zero")`) {
		t.Fatalf("the int zero guard should raise the integer-division message:\n%s", src)
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
	for op, want := range map[Op]string{OpAdd: "+", OpSub: "-", OpMul: "*", OpDiv: "/", OpFloorDiv: "//"} {
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
