package emit

import (
	"testing"

	"github.com/tamnd/unagi/pkg/types"
)

func TestReprScalars(t *testing.T) {
	in := types.NewInterner()
	cases := []struct {
		typ    *types.Type
		goType string
		scalar Scalar
		total  bool
	}{
		{in.Bool(), "bool", SBool, true},
		{in.Int(), "int64", SInt, false},
		{in.Float(), "float64", SFloat, true},
		{in.Str(), "string", SStr, true},
	}
	for _, c := range cases {
		r, ok := Of(c.typ)
		if !ok {
			t.Fatalf("%s should have a representation", c.typ)
		}
		if r.Go != c.goType || r.Scalar != c.scalar || r.Total != c.total {
			t.Fatalf("%s: got {%s %s total=%v}, want {%s %s total=%v}",
				c.typ, r.Go, r.Scalar, r.Total, c.goType, c.scalar, c.total)
		}
	}
}

func TestReprIntIsNotTotal(t *testing.T) {
	in := types.NewInterner()
	r, _ := Of(in.Int())
	if r.Total {
		t.Fatal("int arithmetic guards overflow, so its representation is not total")
	}
}

func TestReprListOfFloat(t *testing.T) {
	in := types.NewInterner()
	r, ok := Of(in.List(in.Float()))
	if !ok {
		t.Fatal("list[float] should lower to []float64")
	}
	if r.Go != "[]float64" || r.Scalar != NotScalar {
		t.Fatalf("got {%s %s}, want []float64 aggregate", r.Go, r.Scalar)
	}
	if r.Elem == nil || r.Elem.Scalar != SFloat {
		t.Fatalf("list element should carry the float representation, got %+v", r.Elem)
	}
}

func TestReprRejectsBoxedTypes(t *testing.T) {
	in := types.NewInterner()
	// A dict has no scalar representation this slice lowers; a list of a boxed
	// element likewise falls through to the boxed tier.
	if _, ok := Of(in.Dict(in.Str(), in.Int())); ok {
		t.Fatal("dict should have no static scalar representation yet")
	}
	if _, ok := Of(in.List(in.Dict(in.Str(), in.Int()))); ok {
		t.Fatal("a list of a boxed element should not lower")
	}
}

func TestScalarStrings(t *testing.T) {
	for s, want := range map[Scalar]string{
		NotScalar: "aggregate", SBool: "bool", SInt: "int", SFloat: "float", SStr: "str",
	} {
		if s.String() != want {
			t.Fatalf("Scalar(%d).String() = %q, want %q", s, s.String(), want)
		}
	}
}
