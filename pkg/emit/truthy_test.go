package emit

import (
	"strings"
	"testing"
)

// This file pins the shared-truthiness invariant (milestones/M4/05 line 37): one
// lowering, truthyExpr, defines what falsy means for a scalar, and every site that
// tests a scalar as a condition goes through it, so a scalar has one notion of falsy
// everywhere. The emit tier tests a scalar as a condition at two sites, `if` and
// `while`; both call truthyExpr, so the emitted test is byte-identical between them.

// TestTruthinessSharedAcrossIfAndWhile renders an if and a while over the same scalar
// condition and asserts the two emit the identical Go test. If the two sites had drifted
// to separate truthiness rules, one scalar would read falsy two different ways and this
// would catch it.
func TestTruthinessSharedAcrossIfAndWhile(t *testing.T) {
	fR, iR, _ := reprs()
	sR := strR()
	cases := []struct {
		name string
		repr Repr
		test string
	}{
		{"int", iR, "n != 0"},
		{"float", fR, "n != 0.0"},
		{"str", sR, `n != ""`},
		{"bool", boolRepr(), "n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cond := Var{Name: "n", Repr: c.repr}
			ifSrc, err := EmitFunc(Func{
				Name:   "f",
				Params: []Param{{Name: "n", Repr: c.repr}},
				Ret:    iR,
				Body:   []Stmt{If{Cond: cond, Then: []Stmt{Return{Value: Int{V: 1}}}}, Return{Value: Int{V: 0}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			whileSrc, err := EmitFunc(Func{
				Name:   "f",
				Params: []Param{{Name: "n", Repr: c.repr}},
				Ret:    iR,
				Body:   []Stmt{While{Cond: cond, Body: []Stmt{Break{}}}, Return{Value: Int{V: 0}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(ifSrc, "if "+c.test+" {") {
				t.Fatalf("the if should test %q:\n%s", c.test, ifSrc)
			}
			if !strings.Contains(whileSrc, "for "+c.test+" {") {
				t.Fatalf("the while should test the same %q:\n%s", c.test, whileSrc)
			}
		})
	}
}

// TestTruthinessRefusesUntestableRepr proves the single rule refuses a representation
// with no falsy form rather than guess one, so a site cannot silently invent a test for
// a type the rule does not cover.
func TestTruthinessRefusesUntestableRepr(t *testing.T) {
	if _, err := truthyExpr(ident("x"), Repr{Scalar: NotScalar}); err == nil {
		t.Fatal("a representation with no truthiness form must be refused, not guessed")
	}
}
