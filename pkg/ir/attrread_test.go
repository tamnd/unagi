package ir

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/emit"
)

// pointShapes is a ShapeResolver that knows one class, Point, with an int field
// x and a float field y, standing in for the build's real shape resolver. Every
// other name has no shape and stays boxed.
func pointShapes(name string) (emit.Repr, bool) {
	if name != "Point" {
		return emit.Repr{}, false
	}
	shape := &emit.Shape{
		Name: "Point",
		Fields: []emit.ShapeField{
			{Name: "x", Repr: emit.Repr{Go: "int64", Scalar: emit.SInt}},
			{Name: "y", Repr: emit.Repr{Go: "float64", Scalar: emit.SFloat, Total: true}},
		},
	}
	return emit.Repr{Go: "Point", Shape: shape}, true
}

// TestLowerAttributeReadsStructField checks that a read of a fixed-shape
// parameter's field lowers to a plain Go struct field load: the parameter takes
// the struct type, the read is p.x with no boxed call, and the result is the
// field's scalar representation. The shape guard that admits the receiver fires
// once at the boxed-to-static entry, so the field load itself carries no guard
// and opens no deopt site of its own.
func TestLowerAttributeReadsStructField(t *testing.T) {
	fn := parseFunc(t, "def get_x(p: Point) -> int:\n    return p.x\n")
	f, err := LowerFuncFull(fn, nil, nil, pointShapes)
	if err != nil {
		t.Fatalf("LowerFuncFull: %v", err)
	}
	got, err := emit.EmitFunc(f)
	if err != nil {
		t.Fatalf("EmitFunc: %v", err)
	}
	for _, want := range []string{
		"func get_x(p Point) (int64, error)",
		"return p.x, nil",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("emitted attribute read is missing %q:\n%s", want, got)
		}
	}
	for _, absent := range []string{"objects.", "LoadAttr", "get_x_deopt"} {
		if strings.Contains(got, absent) {
			t.Errorf("emitted attribute read should not contain %q:\n%s", absent, got)
		}
	}
}

// TestLowerAttributeOnFloatFieldTypesTheReturn checks that the field's own
// representation flows through: reading the float field y makes the function
// return float64, so the shape's per-field types drive the static result rather
// than a single fixed scalar.
func TestLowerAttributeOnFloatFieldTypesTheReturn(t *testing.T) {
	fn := parseFunc(t, "def get_y(p: Point) -> float:\n    return p.y\n")
	f, err := LowerFuncFull(fn, nil, nil, pointShapes)
	if err != nil {
		t.Fatalf("LowerFuncFull: %v", err)
	}
	got, err := emit.EmitFunc(f)
	if err != nil {
		t.Fatalf("EmitFunc: %v", err)
	}
	for _, want := range []string{
		"func get_y(p Point) (float64, error)",
		"return p.y, nil",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("emitted float field read is missing %q:\n%s", want, got)
		}
	}
}

// TestLowerAttributeWithoutShapeResolverRefuses checks that with no shape
// resolver a class-annotated parameter has no static form, so the unit stays
// boxed exactly as it did before the objects tier landed.
func TestLowerAttributeWithoutShapeResolverRefuses(t *testing.T) {
	fn := parseFunc(t, "def get_x(p: Point) -> int:\n    return p.x\n")
	if _, err := LowerFuncFull(fn, nil, nil, nil); err == nil {
		t.Fatal("LowerFuncFull lowered a class parameter with no shape resolver, want refusal")
	}
}

// TestLowerAttributeRejectsUnknownField checks that a read of a field the shape
// does not list has no static form and keeps the unit boxed, rather than loading
// a Go field that the struct does not carry.
func TestLowerAttributeRejectsUnknownField(t *testing.T) {
	fn := parseFunc(t, "def get_z(p: Point) -> int:\n    return p.z\n")
	if _, err := LowerFuncFull(fn, nil, nil, pointShapes); err == nil {
		t.Fatal("LowerFuncFull lowered a read of an unknown field, want refusal")
	}
}
