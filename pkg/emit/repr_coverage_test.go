package emit

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"testing"

	"github.com/tamnd/unagi/pkg/types"
)

// This file fills the representation-table checklist (milestones/M4/01): the list
// representations beyond list[float], the nested-aggregate refusal, and the zero
// values the error-return path emits. The existing repr_test.go covers the scalar
// core and list[float]; these cases are the ones a wrong entry would miscompile
// silently, so each pins its exact Go form.

// printExpr renders a bare expression node to source, so a test can assert on a
// zero value or a goType node without wrapping it in a function first.
func printExpr(t *testing.T, e ast.Expr) string {
	t.Helper()
	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), e); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// TestReprScalarListElements checks every scalar list lowers to the right Go slice
// with the element representation carried and the list itself an aggregate, so the
// range and index paths downstream read the element type from Elem.
func TestReprScalarListElements(t *testing.T) {
	in := types.NewInterner()
	cases := []struct {
		elem    *types.Type
		wantGo  string
		wantScl Scalar
	}{
		{in.Int(), "[]int64", SInt},
		{in.Float(), "[]float64", SFloat},
		{in.Bool(), "[]bool", SBool},
		{in.Str(), "[]string", SStr},
	}
	for _, c := range cases {
		r, ok := Of(in.List(c.elem))
		if !ok {
			t.Fatalf("list of %s should lower", c.elem)
		}
		if r.Go != c.wantGo {
			t.Fatalf("list of %s: Go = %q, want %q", c.elem, r.Go, c.wantGo)
		}
		if r.Scalar != NotScalar {
			t.Fatalf("list of %s: a list is an aggregate, got scalar %s", c.elem, r.Scalar)
		}
		if r.Elem == nil || r.Elem.Scalar != c.wantScl {
			t.Fatalf("list of %s: element repr not carried", c.elem)
		}
		if got := printExpr(t, r.goType()); got != c.wantGo {
			t.Fatalf("list of %s: goType prints %q, want %q", c.elem, got, c.wantGo)
		}
	}
}

// TestReprRejectsNestedList proves an aggregate element keeps the whole list boxed:
// list[list[int]] has no static form, because the nested list is NotScalar and the
// table refuses it rather than lowering to a wrong shape.
func TestReprRejectsNestedList(t *testing.T) {
	in := types.NewInterner()
	if _, ok := Of(in.List(in.List(in.Int()))); ok {
		t.Fatal("a list of lists has no static representation and must stay boxed")
	}
}

// TestReprRejectsMalformedListArity proves the `len(elems) != 1` guard in Of: a
// list type that carries no element or more than one has no static representation,
// so Of returns false rather than indexing a missing element (a panic) or silently
// picking the first of several (a wrong shape). A well-formed list always carries
// exactly one element type, so this arity can only come from an inference bug
// reaching emit; the guard turns that bug into a boxed decision, never wrong Go.
func TestReprRejectsMalformedListArity(t *testing.T) {
	in := types.NewInterner()
	if _, ok := Of(in.ListN()); ok {
		t.Error("a list with no element type has no static representation and must stay boxed")
	}
	if _, ok := Of(in.ListN(in.Int(), in.Str())); ok {
		t.Error("a list with more than one element type has no static representation and must stay boxed")
	}
}

// TestReprZeroValues pins the Go zero each representation returns on the error path,
// the first result of a (T, error) bail before a real value exists. The float zero
// must keep its decimal point so the error return types float, not int.
func TestReprZeroValues(t *testing.T) {
	in := types.NewInterner()
	cases := []struct {
		t    *types.Type
		want string
	}{
		{in.Int(), "0"},
		{in.Float(), "0.0"},
		{in.Bool(), "false"},
		{in.Str(), `""`},
	}
	for _, c := range cases {
		r, ok := Of(c.t)
		if !ok {
			t.Fatalf("%s should lower", c.t)
		}
		if got := printExpr(t, r.zero()); got != c.want {
			t.Fatalf("%s zero = %q, want %q", c.t, got, c.want)
		}
	}
	// A list's zero is the nil slice, the aggregate branch of zero().
	lr, _ := Of(in.List(in.Float()))
	if got := printExpr(t, lr.zero()); got != "nil" {
		t.Fatalf("list zero = %q, want nil", got)
	}
}
