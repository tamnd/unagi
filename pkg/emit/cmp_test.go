package emit

import (
	"strings"
	"testing"
)

func strR() Repr { return Repr{Go: "string", Scalar: SStr, Total: true} }
func bR() Repr   { return boolRepr() }

func TestStringConcat(t *testing.T) {
	src := emitOneReturn(t, "greet", strR(), []Param{{Name: "a", Repr: strR()}, {Name: "b", Repr: strR()}},
		Bin{Op: OpAdd, L: Var{Name: "a", Repr: strR()}, R: Var{Name: "b", Repr: strR()}})
	if !strings.Contains(src, "return a + b, nil") {
		t.Fatalf("string concat should be a bare +:\n%s", src)
	}
}

func TestStringOnlyConcatenates(t *testing.T) {
	_, err := EmitFunc(Func{
		Name: "bad", Ret: strR(),
		Body: []Stmt{Return{Value: Bin{Op: OpSub, L: Str{V: "a"}, R: Str{V: "b"}}}},
	})
	if err == nil {
		t.Fatal("subtracting strings is not a static operation and should be refused")
	}
}

func TestNumberComparisonYieldsBool(t *testing.T) {
	_, iR, _ := reprs()
	src := emitOneReturn(t, "lt", bR(), []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}},
		Cmp{Op: CmpLt, L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: iR}})
	if !strings.Contains(src, "(bool, error)") {
		t.Fatalf("a comparison should return bool:\n%s", src)
	}
	if !strings.Contains(src, "return a < b, nil") {
		t.Fatalf("comparison should be a bare <:\n%s", src)
	}
}

func TestMixedComparisonCoerces(t *testing.T) {
	fR, iR, _ := reprs()
	src := emitOneReturn(t, "ge", bR(), []Param{{Name: "n", Repr: iR}, {Name: "x", Repr: fR}},
		Cmp{Op: CmpGe, L: Var{Name: "n", Repr: iR}, R: Var{Name: "x", Repr: fR}})
	if !strings.Contains(src, "return float64(n) >= x, nil") {
		t.Fatalf("a mixed comparison should coerce the int side:\n%s", src)
	}
}

func TestStringComparison(t *testing.T) {
	src := emitOneReturn(t, "eq", bR(), []Param{{Name: "a", Repr: strR()}, {Name: "b", Repr: strR()}},
		Cmp{Op: CmpEq, L: Var{Name: "a", Repr: strR()}, R: Var{Name: "b", Repr: strR()}})
	if !strings.Contains(src, "return a == b, nil") {
		t.Fatalf("strings should compare directly:\n%s", src)
	}
}

func TestBoolOrderingRejected(t *testing.T) {
	_, err := EmitFunc(Func{
		Name: "bad", Ret: bR(),
		Body: []Stmt{Return{Value: Cmp{Op: CmpLt, L: Bool{V: true}, R: Bool{V: false}}}},
	})
	if err == nil {
		t.Fatal("ordering two bools is not a static operation and should be refused")
	}
}

func TestBooleanConnectives(t *testing.T) {
	src := emitOneReturn(t, "both", bR(), []Param{{Name: "p", Repr: bR()}, {Name: "q", Repr: bR()}},
		And{L: Var{Name: "p", Repr: bR()}, R: Var{Name: "q", Repr: bR()}})
	if !strings.Contains(src, "return p && q, nil") {
		t.Fatalf("and should lower to &&:\n%s", src)
	}
}

func TestConnectivePrecedenceParens(t *testing.T) {
	// (p or q) and r must keep its grouping, since Go binds && tighter than ||.
	src := emitOneReturn(t, "grp", bR(), []Param{{Name: "p", Repr: bR()}, {Name: "q", Repr: bR()}, {Name: "r", Repr: bR()}},
		And{L: Or{L: Var{Name: "p", Repr: bR()}, R: Var{Name: "q", Repr: bR()}}, R: Var{Name: "r", Repr: bR()}})
	if !strings.Contains(src, "return (p || q) && r, nil") {
		t.Fatalf("the or operand of an and must be parenthesized:\n%s", src)
	}
}

func TestNotParenthesizesBinary(t *testing.T) {
	src := emitOneReturn(t, "neither", bR(), []Param{{Name: "p", Repr: bR()}, {Name: "q", Repr: bR()}},
		Not{X: And{L: Var{Name: "p", Repr: bR()}, R: Var{Name: "q", Repr: bR()}}})
	if !strings.Contains(src, "return !(p && q), nil") {
		t.Fatalf("not of a connective must be parenthesized:\n%s", src)
	}
}

func TestNotNeedsBool(t *testing.T) {
	_, iR, _ := reprs()
	_, err := EmitFunc(Func{
		Name: "bad", Ret: bR(),
		Body: []Stmt{Return{Value: Not{X: Var{Name: "n", Repr: iR}}}},
	})
	if err == nil {
		t.Fatal("not on an int operand should be refused")
	}
}

func TestCmpOpStrings(t *testing.T) {
	for op, want := range map[CmpOp]string{
		CmpLt: "<", CmpLe: "<=", CmpGt: ">", CmpGe: ">=", CmpEq: "==", CmpNe: "!=",
	} {
		if op.String() != want {
			t.Fatalf("CmpOp(%d).String() = %q, want %q", op, op.String(), want)
		}
	}
}
