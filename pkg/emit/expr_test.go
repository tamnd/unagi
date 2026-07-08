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

func TestIntAddAssignIsGuarded(t *testing.T) {
	_, iR, _ := reprs()
	// An int accumulator lowers through the guarded add, not a bare +=, so it
	// cannot wrap silently.
	got, err := EmitFunc(Func{
		Name:   "acc",
		Params: []Param{{Name: "n", Repr: iR}},
		Ret:    iR,
		Body: []Stmt{
			Define{Name: "s", Value: Int{V: 0}},
			AddAssign{Name: "s", Repr: iR, Value: Var{Name: "n", Repr: iR}},
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
	for op, want := range map[Op]string{OpAdd: "+", OpSub: "-", OpMul: "*", OpDiv: "/"} {
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
