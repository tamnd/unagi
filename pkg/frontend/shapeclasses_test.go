package frontend

import (
	"testing"
)

// shapesOf parses a module and returns its shape classes keyed by name, so a test
// can assert on membership and field layout without caring about the sorted slice
// order.
func shapesOf(t *testing.T, src string) map[string]ShapeClass {
	t.Helper()
	mod, err := Parse([]byte(src), "shapeclasses_test.py")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := map[string]ShapeClass{}
	for _, c := range ShapeClasses(mod) {
		out[c.Name] = c
	}
	return out
}

// TestShapeClassFixedSlots checks that a __slots__ class with a scalar annotation
// per slot qualifies, and that the fields come back in __slots__ order with their
// annotated types.
func TestShapeClassFixedSlots(t *testing.T) {
	shapes := shapesOf(t, `
class Point:
    __slots__ = ("x", "y")
    x: int
    y: float

    def __init__(self, x: int, y: float) -> None:
        self.x = x
        self.y = y
`)
	p, ok := shapes["Point"]
	if !ok {
		t.Fatalf("Point should qualify as a fixed shape, got %v", shapes)
	}
	if len(p.Fields) != 2 {
		t.Fatalf("Point should have two fields, got %v", p.Fields)
	}
	if p.Fields[0] != (ShapeSlot{Name: "x", Type: "int"}) || p.Fields[1] != (ShapeSlot{Name: "y", Type: "float"}) {
		t.Fatalf("fields should follow __slots__ order with annotated types, got %v", p.Fields)
	}
}

// TestShapeClassListSlots checks that a list-form __slots__ qualifies the same way
// a tuple does, since both fix the slot set.
func TestShapeClassListSlots(t *testing.T) {
	shapes := shapesOf(t, `
class Flags:
    __slots__ = ["on", "name"]
    on: bool
    name: str
`)
	f, ok := shapes["Flags"]
	if !ok {
		t.Fatalf("a list __slots__ should qualify, got %v", shapes)
	}
	if f.Fields[0].Type != "bool" || f.Fields[1].Type != "str" {
		t.Fatalf("fields should carry the annotated scalar types, got %v", f.Fields)
	}
}

// TestShapeClassRejectsMissingAnnotation checks that a slot with no scalar
// annotation leaves the whole class boxed, since the field has no fixed
// representation.
func TestShapeClassRejectsMissingAnnotation(t *testing.T) {
	shapes := shapesOf(t, `
class Point:
    __slots__ = ("x", "y")
    x: int
`)
	if _, ok := shapes["Point"]; ok {
		t.Fatalf("a slot without a scalar annotation should disqualify the class, got %v", shapes)
	}
}

// TestShapeClassRejectsNoSlots checks that a class without __slots__ never
// qualifies: its instances carry a __dict__ and can gain attributes, so the shape
// is not fixed.
func TestShapeClassRejectsNoSlots(t *testing.T) {
	shapes := shapesOf(t, `
class Point:
    x: int
    y: int
`)
	if _, ok := shapes["Point"]; ok {
		t.Fatalf("a class without __slots__ has an open layout and should stay boxed, got %v", shapes)
	}
}

// TestShapeClassRejectsNonScalarSlot checks that a non-scalar annotation (a list,
// a user type) disqualifies the class, since only scalar fields lower to a plain
// struct field at this tier.
func TestShapeClassRejectsNonScalarSlot(t *testing.T) {
	shapes := shapesOf(t, `
class Bag:
    __slots__ = ("items",)
    items: list
`)
	if _, ok := shapes["Bag"]; ok {
		t.Fatalf("a non-scalar slot should keep the class boxed, got %v", shapes)
	}
}

// TestShapeClassRejectsBase checks that any base other than object disqualifies
// the class, since a base could contribute an unmodeled slot.
func TestShapeClassRejectsBase(t *testing.T) {
	shapes := shapesOf(t, `
class Base:
    __slots__ = ("a",)
    a: int


class Derived(Base):
    __slots__ = ("b",)
    b: int
`)
	if _, ok := shapes["Derived"]; ok {
		t.Fatalf("a class with a non-object base should stay boxed, got %v", shapes)
	}
	if _, ok := shapes["Base"]; !ok {
		t.Fatalf("the object-based Base should still qualify, got %v", shapes)
	}
}

// TestShapeClassAllowsObjectBase checks that spelling the object base explicitly
// still qualifies, since it adds no slot.
func TestShapeClassAllowsObjectBase(t *testing.T) {
	shapes := shapesOf(t, `
class Point(object):
    __slots__ = ("x",)
    x: int
`)
	if _, ok := shapes["Point"]; !ok {
		t.Fatalf("an explicit object base should still qualify, got %v", shapes)
	}
}

// TestShapeClassRejectsDecorator checks that a class decorator disqualifies the
// shape, since the decorator can return a differently shaped object.
func TestShapeClassRejectsDecorator(t *testing.T) {
	shapes := shapesOf(t, `
import dataclasses


@dataclasses.dataclass
class Point:
    __slots__ = ("x",)
    x: int
`)
	if _, ok := shapes["Point"]; ok {
		t.Fatalf("a decorated class should stay boxed, got %v", shapes)
	}
}

// TestShapeClassRejectsMetaclassKeyword checks that a class keyword (a metaclass
// or other class argument) disqualifies the shape.
func TestShapeClassRejectsMetaclassKeyword(t *testing.T) {
	shapes := shapesOf(t, `
class Meta(type):
    __slots__ = ()


class Point(metaclass=Meta):
    __slots__ = ("x",)
    x: int
`)
	if _, ok := shapes["Point"]; ok {
		t.Fatalf("a class with a metaclass keyword should stay boxed, got %v", shapes)
	}
}

// TestShapeClassRejectsReboundName checks that a class whose name is bound a
// second time in the module is disqualified, since a later binding could swap a
// differently shaped object in under the same name.
func TestShapeClassRejectsReboundName(t *testing.T) {
	shapes := shapesOf(t, `
class Point:
    __slots__ = ("x",)
    x: int


Point = None
`)
	if _, ok := shapes["Point"]; ok {
		t.Fatalf("a rebound class name should stay boxed, got %v", shapes)
	}
}

// TestShapeClassRejectsNestedClass checks that a class nested in a function is not
// a module-level shape, since its definition does not sit in module scope.
func TestShapeClassRejectsNestedClass(t *testing.T) {
	shapes := shapesOf(t, `
def make():
    class Point:
        __slots__ = ("x",)
        x: int

    return Point
`)
	if _, ok := shapes["Point"]; ok {
		t.Fatalf("a class nested in a function should not be a module shape, got %v", shapes)
	}
}

// TestShapeClassRejectsDunderSlot checks that a __dict__ or __weakref__ slot
// reopens a dynamic layout and disqualifies the class.
func TestShapeClassRejectsDunderSlot(t *testing.T) {
	shapes := shapesOf(t, `
class Point:
    __slots__ = ("x", "__dict__")
    x: int
`)
	if _, ok := shapes["Point"]; ok {
		t.Fatalf("a __dict__ slot reopens the layout and should stay boxed, got %v", shapes)
	}
}

// TestShapeClassEmptyModule checks that a module with no qualifying class returns
// nil, leaving the boxed tier untouched.
func TestShapeClassEmptyModule(t *testing.T) {
	mod, err := Parse([]byte("x = 1\n"), "shapeclasses_test.py")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := ShapeClasses(mod); got != nil {
		t.Fatalf("a module with no shape class should return nil, got %v", got)
	}
}
