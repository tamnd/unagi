package ir

import (
	"testing"

	"github.com/tamnd/unagi/pkg/emit"
	"github.com/tamnd/unagi/pkg/frontend"
)

// parseModule parses a whole module for the shape-table tests, which need the
// class declarations the single-def helper drops.
func parseModule(t *testing.T, src string) *frontend.Module {
	t.Helper()
	m, err := frontend.Parse([]byte(src), "test.py")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return m
}

const pointModule = `class Point:
    __slots__ = ("x", "y")
    x: int
    y: float
`

func TestTrackedShapesBuildsStructRepr(t *testing.T) {
	m := parseModule(t, pointModule)
	tracked := TrackedShapes(m)
	r, ok := tracked["Point"]
	if !ok {
		t.Fatalf("Point not tracked; got %v", tracked)
	}
	if r.Go != "Point" {
		t.Fatalf("struct Go type = %q, want Point", r.Go)
	}
	if r.Shape == nil {
		t.Fatalf("tracked Point has nil Shape")
	}
	if r.Shape.Name != "Point" {
		t.Fatalf("shape name = %q, want Point", r.Shape.Name)
	}
	want := []emit.ShapeField{
		{Name: "x", Repr: emit.Repr{Go: "int64", Scalar: emit.SInt}},
		{Name: "y", Repr: emit.Repr{Go: "float64", Scalar: emit.SFloat}},
	}
	if len(r.Shape.Fields) != len(want) {
		t.Fatalf("shape has %d fields, want %d: %v", len(r.Shape.Fields), len(want), r.Shape.Fields)
	}
	for i, w := range want {
		got := r.Shape.Fields[i]
		if got.Name != w.Name || got.Repr.Go != w.Repr.Go || got.Repr.Scalar != w.Repr.Scalar {
			t.Fatalf("field %d = %+v, want %+v", i, got, w)
		}
	}
}

func TestTrackedShapesNilWithoutShapeClass(t *testing.T) {
	m := parseModule(t, "def f(a: int) -> int:\n    return a\n")
	if got := TrackedShapes(m); got != nil {
		t.Fatalf("TrackedShapes = %v, want nil for a module with no shape class", got)
	}
}

func TestShapeResolverForResolvesTrackedClass(t *testing.T) {
	m := parseModule(t, pointModule)
	resolve := ShapeResolverFor(TrackedShapes(m))
	if resolve == nil {
		t.Fatal("resolver is nil for a module with a shape class")
	}
	r, ok := resolve("Point")
	if !ok {
		t.Fatal("resolver refused Point")
	}
	if r.Shape == nil || r.Shape.Name != "Point" {
		t.Fatalf("resolved Point has shape %+v", r.Shape)
	}
	if _, ok := resolve("Missing"); ok {
		t.Fatal("resolver accepted a name that is not a shape class")
	}
}

func TestShapeResolverForNilWhenEmpty(t *testing.T) {
	if got := ShapeResolverFor(nil); got != nil {
		t.Fatal("ShapeResolverFor(nil) should be a nil resolver")
	}
	if got := ShapeResolverFor(map[string]emit.Repr{}); got != nil {
		t.Fatal("ShapeResolverFor(empty) should be a nil resolver")
	}
}
