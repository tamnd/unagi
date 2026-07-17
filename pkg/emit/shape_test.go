package emit

import (
	"testing"
)

// TestEmitShapeGolden proves a fixed-shape class renders to a Go struct type: the
// class name as the struct type, one field per slot in slot order, each typed by
// its scalar representation. This is the layout a static form types an instance
// against and an attribute read loads a field of.
func TestEmitShapeGolden(t *testing.T) {
	shape := Shape{
		Name: "Point",
		Fields: []ShapeField{
			{Name: "x", Repr: Repr{Go: "int64", Scalar: SInt}},
			{Name: "y", Repr: Repr{Go: "float64", Scalar: SFloat}},
		},
	}
	got, err := EmitShape(shape)
	if err != nil {
		t.Fatal(err)
	}
	want := `type Point struct {
	x int64
	y float64
}`
	if got != want {
		t.Errorf("EmitShape mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestEmitShapeEmptyStruct proves a shape with no scalar-typed field still renders
// a well-formed empty struct, so the emitter never produces broken Go for a
// degenerate layout.
func TestEmitShapeEmptyStruct(t *testing.T) {
	got, err := EmitShape(Shape{Name: "Empty"})
	if err != nil {
		t.Fatal(err)
	}
	want := "type Empty struct {\n}"
	if got != want {
		t.Errorf("EmitShape empty mismatch:\ngot:\n%q\nwant:\n%q", got, want)
	}
}
