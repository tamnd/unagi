package emit

import (
	"strings"
	"testing"
)

// This file covers the branch-join declaration (06_statements_control_flow.md lines
// 12 and 33). A name both arms of an if/else bind is declared once ahead of the
// branch with its definite Go type, and each arm reassigns it, so the value read
// after the block is whichever arm ran and no untyped Go zero leaks past the join.

func TestVarDeclHoistsTypedJoinLocal(t *testing.T) {
	_, iR, _ := reprs()
	// `var x int64` ahead of the if, each arm `x = ...`, then `return x`: the join
	// local is declared once and reassigned per arm, never redeclared inside a block.
	src, err := EmitFunc(Func{
		Name:   "f",
		Params: []Param{{Name: "c", Repr: iR}},
		Ret:    iR,
		Body: []Stmt{
			VarDecl{Name: "x", Repr: iR},
			If{
				Cond: Cmp{Op: CmpGt, L: Var{Name: "c", Repr: iR}, R: Int{V: 0}},
				Then: []Stmt{Assign{Name: "x", Value: Int{V: 10}}},
				Else: []Stmt{Assign{Name: "x", Value: Int{V: 20}}},
			},
			Return{Value: Var{Name: "x", Repr: iR}},
		},
	})
	if err != nil {
		t.Fatalf("EmitFunc: %v", err)
	}
	if !strings.Contains(src, "var x int64") {
		t.Fatalf("the join name should be declared with its Go type ahead of the branch:\n%s", src)
	}
	decl := strings.Index(src, "var x int64")
	branch := strings.Index(src, "if c > 0 {")
	if decl < 0 || branch < 0 || decl > branch {
		t.Fatalf("the declaration should sit ahead of the branch:\n%s", src)
	}
	if !strings.Contains(src, "x = 10") || !strings.Contains(src, "x = 20") {
		t.Fatalf("each arm should reassign the hoisted local:\n%s", src)
	}
	if strings.Contains(src, "x :=") {
		t.Fatalf("a join name must never be redeclared with := inside an arm:\n%s", src)
	}
}
