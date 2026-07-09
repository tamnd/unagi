package build

import (
	"testing"

	"github.com/tamnd/unagi/pkg/frontend"
)

// These tests exercise the resume shape proof directly on synthesized frontend
// ASTs, so they cover the gate without compiling a binary. The end-to-end
// behavior lives in build_test.go; here we pin exactly which shapes earn a
// mid-loop resume and which keep the from-top deopt edge.

func name(id string) *frontend.Name { return &frontend.Name{Id: id} }

func intLit(text string) *frontend.IntLit { return &frontend.IntLit{Text: text} }

// rangeCall builds `range(args...)` with plain positional arguments.
func rangeCall(args ...frontend.Expr) *frontend.Call {
	as := make([]frontend.Arg, len(args))
	for i, a := range args {
		as[i] = frontend.Arg{Value: a}
	}
	return &frontend.Call{Fn: name("range"), Args: as}
}

// mulAcc is `acc = acc * 2`, the canonical single guarded update.
func mulAcc(acc string) *frontend.Assign {
	return &frontend.Assign{
		Targets: []frontend.Expr{name(acc)},
		Value:   &frontend.BinOp{Left: name(acc), Op: frontend.BinMul, Right: intLit("2")},
	}
}

// canonicalDef builds `def f(n): acc = 1; for i in range(n): acc = acc * 2; return acc`.
func canonicalDef(loopVar, acc string) *frontend.FuncDef {
	return &frontend.FuncDef{
		Name:   "f",
		Params: []frontend.Param{{Name: "n", Kind: frontend.ParamPlain}},
		Body: []frontend.Stmt{
			&frontend.Assign{Targets: []frontend.Expr{name(acc)}, Value: intLit("1")},
			&frontend.For{
				Target: name(loopVar),
				Iter:   rangeCall(name("n")),
				Body:   []frontend.Stmt{mulAcc(acc)},
			},
			&frontend.Return{Value: name(acc)},
		},
	}
}

func TestResumeShapeAcceptsCanonicalLoop(t *testing.T) {
	shape, ok := resumeShapeFor(canonicalDef("i", "total"))
	if !ok {
		t.Fatal("canonical single-accumulator counting loop should resume")
	}
	if shape.loopVar != "i" || shape.acc != "total" {
		t.Errorf("shape = {loopVar %q, acc %q}, want {i, total}", shape.loopVar, shape.acc)
	}
	if shape.twin == nil {
		t.Fatal("shape carries no twin")
	}
	// The twin promotes the counter and the accumulator to leading parameters ahead
	// of the original parameters, so the hand-off can seed them.
	if len(shape.twin.Params) != 3 ||
		shape.twin.Params[0].Name != "i" ||
		shape.twin.Params[1].Name != "total" ||
		shape.twin.Params[2].Name != "n" {
		t.Errorf("twin params = %v, want [i total n]", paramNames(shape.twin))
	}
	// The twin restarts the loop from the counter: `for i in range(i, n)`.
	loop, ok := shape.twin.Body[0].(*frontend.For)
	if !ok {
		t.Fatalf("twin body[0] = %T, want *For", shape.twin.Body[0])
	}
	call := loop.Iter.(*frontend.Call)
	if len(call.Args) != 2 {
		t.Fatalf("twin range has %d args, want 2 (seeded start, stop)", len(call.Args))
	}
	if start, ok := call.Args[0].Value.(*frontend.Name); !ok || start.Id != "i" {
		t.Errorf("twin range start = %v, want the loop counter i", call.Args[0].Value)
	}
}

func TestResumeShapeRejectsSecondAccumulator(t *testing.T) {
	// A second accumulator updated in the body before the guard would be re-run by a
	// mid-loop re-entry, double-applying it, so the shape must stay from-top.
	d := canonicalDef("i", "total")
	loop := d.Body[1].(*frontend.For)
	loop.Body = []frontend.Stmt{
		&frontend.Assign{Targets: []frontend.Expr{name("s")}, Value: &frontend.BinOp{
			Left: name("s"), Op: frontend.BinAdd, Right: name("n"),
		}},
		mulAcc("total"),
	}
	if _, ok := resumeShapeFor(d); ok {
		t.Error("a two-statement loop body must not resume")
	}
}

func TestResumeShapeRejectsUnprovenBody(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*frontend.FuncDef)
	}{
		{
			// A descending step is not a monotonic counter the twin can restate.
			name: "descending step",
			mut: func(d *frontend.FuncDef) {
				loop := d.Body[1].(*frontend.For)
				loop.Iter = rangeCall(name("n"), intLit("0"), &frontend.UnaryOp{
					Op: frontend.UnaryNeg, X: intLit("1"),
				})
			},
		},
		{
			// A computed bound would need re-evaluation from state the twin may not hold.
			name: "computed bound",
			mut: func(d *frontend.FuncDef) {
				loop := d.Body[1].(*frontend.For)
				loop.Iter = rangeCall(&frontend.BinOp{
					Left: name("n"), Op: frontend.BinMul, Right: intLit("2"),
				})
			},
		},
		{
			// The body reads an outer local the resume would drop.
			name: "reads outer local",
			mut: func(d *frontend.FuncDef) {
				loop := d.Body[1].(*frontend.For)
				loop.Body = []frontend.Stmt{&frontend.Assign{
					Targets: []frontend.Expr{name("total")},
					Value:   &frontend.BinOp{Left: name("total"), Op: frontend.BinMul, Right: name("other")},
				}}
			},
		},
		{
			// A non-overflowing op opens no deopt edge, so there is nothing to resume.
			name: "non overflowing op",
			mut: func(d *frontend.FuncDef) {
				loop := d.Body[1].(*frontend.For)
				loop.Body = []frontend.Stmt{&frontend.Assign{
					Targets: []frontend.Expr{name("total")},
					Value:   &frontend.BinOp{Left: name("total"), Op: frontend.BinMod, Right: intLit("2")},
				}}
			},
		},
		{
			// A second top-level loop is outside the single-loop proof.
			name: "second loop",
			mut: func(d *frontend.FuncDef) {
				d.Body = append(d.Body, &frontend.For{
					Target: name("j"),
					Iter:   rangeCall(name("n")),
					Body:   []frontend.Stmt{mulAcc("total")},
				})
			},
		},
		{
			// The accumulator is never initialized before the loop, so the twin has no
			// initializer to replace with a parameter.
			name: "accumulator not initialized",
			mut: func(d *frontend.FuncDef) {
				d.Body = d.Body[1:]
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := canonicalDef("i", "total")
			tc.mut(d)
			if _, ok := resumeShapeFor(d); ok {
				t.Errorf("%s should not resume", tc.name)
			}
		})
	}
}
