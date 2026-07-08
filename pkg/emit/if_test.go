package emit

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/types"
)

// This file covers the if lowering and the truthiness rule it shares with while
// and the connectives (milestones/M4/05 lines 32-36 and 06 lines 30-32): each
// scalar condition lowers to the Go test its type calls falsy, an elif folds to
// else-if, and a guarded condition flushes its guard ahead of the if rather than
// into the tested expression.

// emitIf builds a one-parameter function whose body is a single if and returns its
// emitted source, the small harness the truthiness cases reuse.
func emitIf(t *testing.T, param Repr, cond Expr, then, els []Stmt) string {
	t.Helper()
	src, err := EmitFunc(Func{
		Name:   "f",
		Params: []Param{{Name: "n", Repr: param}},
		Ret:    param,
		Body:   []Stmt{If{Cond: cond, Then: then, Else: els}},
	})
	if err != nil {
		t.Fatalf("EmitFunc: %v", err)
	}
	return src
}

func TestIfIntTruthiness(t *testing.T) {
	_, iR, _ := reprs()
	src := emitIf(t, iR,
		Var{Name: "n", Repr: iR},
		[]Stmt{Return{Value: Int{V: 1}}},
		[]Stmt{Return{Value: Int{V: 0}}})
	if !strings.Contains(src, "if n != 0 {") {
		t.Fatalf("an int condition should test against zero:\n%s", src)
	}
}

func TestIfFloatTruthiness(t *testing.T) {
	fR, _, _ := reprs()
	src := emitIf(t, fR,
		Var{Name: "n", Repr: fR},
		[]Stmt{Return{Value: Float{V: 1}}},
		[]Stmt{Return{Value: Float{V: 0}}})
	if !strings.Contains(src, "if n != 0.0 {") {
		t.Fatalf("a float condition should test against 0.0:\n%s", src)
	}
}

func TestIfStrTruthiness(t *testing.T) {
	sR := strR()
	src := emitIf(t, sR,
		Var{Name: "n", Repr: sR},
		[]Stmt{Return{Value: Str{V: "yes"}}},
		[]Stmt{Return{Value: Str{V: "no"}}})
	if !strings.Contains(src, `if n != "" {`) {
		t.Fatalf("a str condition should test against the empty string:\n%s", src)
	}
}

func TestIfBoolTruthinessIsDirect(t *testing.T) {
	src := emitIf(t, bR(),
		Var{Name: "n", Repr: bR()},
		[]Stmt{Return{Value: Bool{V: true}}},
		[]Stmt{Return{Value: Bool{V: false}}})
	if !strings.Contains(src, "if n {") {
		t.Fatalf("a bool condition should stand on its own:\n%s", src)
	}
	if strings.Contains(src, "n != ") || strings.Contains(src, "n == ") {
		t.Fatalf("a bool condition should not be compared to anything:\n%s", src)
	}
}

// TestIfListTruthiness proves the aggregate arm of the shared rule: a list is
// falsy when empty, which lowers to a length test. This is the emit-level proof of
// 05 line 35; the bridge carries no list condition operand yet.
func TestIfListTruthiness(t *testing.T) {
	in := types.NewInterner()
	listIntR, _ := Of(in.List(in.Int()))
	intR, _ := Of(in.Int())
	src, err := EmitFunc(Func{
		Name:   "f",
		Params: []Param{{Name: "xs", Repr: listIntR}},
		Ret:    intR,
		Body: []Stmt{If{
			Cond: Var{Name: "xs", Repr: listIntR},
			Then: []Stmt{Return{Value: Int{V: 1}}},
			Else: []Stmt{Return{Value: Int{V: 0}}},
		}},
	})
	if err != nil {
		t.Fatalf("EmitFunc: %v", err)
	}
	if !strings.Contains(src, "if len(xs) != 0 {") {
		t.Fatalf("a list condition should test its length:\n%s", src)
	}
}

// TestElifFoldsToElseIf proves the elif chain (06 line 31): a nested guard-free if
// in the else arm prints as `else if`, not a braced `else { if }`.
func TestElifFoldsToElseIf(t *testing.T) {
	_, iR, _ := reprs()
	src, err := EmitFunc(Func{
		Name:   "sign",
		Params: []Param{{Name: "x", Repr: iR}},
		Ret:    iR,
		Body: []Stmt{If{
			Cond: Cmp{Op: CmpGt, L: Var{Name: "x", Repr: iR}, R: Int{V: 0}},
			Then: []Stmt{Return{Value: Int{V: 1}}},
			Else: []Stmt{If{
				Cond: Cmp{Op: CmpLt, L: Var{Name: "x", Repr: iR}, R: Int{V: 0}},
				Then: []Stmt{Return{Value: Int{V: -1}}},
				Else: []Stmt{Return{Value: Int{V: 0}}},
			}},
		}},
	})
	if err != nil {
		t.Fatalf("EmitFunc: %v", err)
	}
	if !strings.Contains(src, "} else if x < 0 {") {
		t.Fatalf("a nested if in the else arm should fold to else if:\n%s", src)
	}
	if strings.Contains(src, "} else {\n\t\tif ") {
		t.Fatalf("the elif should not print as a braced else block:\n%s", src)
	}
}

// TestIfGuardedConditionFlushesAhead proves 06 line 32: a guarded int operation in
// the condition emits its overflow check and deopt edge before the if, so the
// tested expression is the already-proven value, never the guarded operation.
func TestIfGuardedConditionFlushesAhead(t *testing.T) {
	_, iR, _ := reprs()
	src, err := EmitFunc(Func{
		Name:   "f",
		Params: []Param{{Name: "a", Repr: iR}, {Name: "b", Repr: iR}},
		Ret:    iR,
		Body: []Stmt{If{
			Cond: Bin{Op: OpAdd, L: Var{Name: "a", Repr: iR}, R: Var{Name: "b", Repr: iR}},
			Then: []Stmt{Return{Value: Int{V: 1}}},
			Else: []Stmt{Return{Value: Int{V: 0}}},
		}},
	})
	if err != nil {
		t.Fatalf("EmitFunc: %v", err)
	}
	add := strings.Index(src, "rt.AddInt64(a, b)")
	guard := strings.Index(src, "return f_deopt0(a, b)")
	test := strings.Index(src, "if t0 != 0 {")
	if add < 0 || guard < 0 || test < 0 {
		t.Fatalf("a guarded condition should bind the add, deopt on overflow, then test the value:\n%s", src)
	}
	if !(add < guard && guard < test) {
		t.Fatalf("the guard should flush ahead of the if, order add<guard<test:\n%s", src)
	}
}

// TestTruthinessRefusesUnrepresentable pins that a value with no truthiness form is
// refused rather than lowered to a wrong test.
func TestTruthinessRefusesUnrepresentable(t *testing.T) {
	if _, err := truthyExpr(ident("x"), Repr{}); err == nil {
		t.Fatal("a representation with no scalar class has no truthiness form and should be refused")
	}
}
