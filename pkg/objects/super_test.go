package objects

import (
	"strings"
	"testing"
)

// method builds a plain function that returns a fixed string, so a test can
// tell which class's method a super walk resolved to.
func method(qual, ret string) *functionObject {
	return NewFunction(qual, []Param{{Name: "self", Kind: ParamPlain}}, nil,
		func(args []Object) (Object, error) { return NewStr(ret), nil }).(*functionObject)
}

func TestSuperWalksInstanceMRO(t *testing.T) {
	// Diamond: Left and Right both define m; from Left, super() must reach
	// Right, not Base, because it walks the instance's linearization.
	base := mkclass(t, "Base")
	base.setAttr("m", method("Base.m", "base"))
	left := mkclass(t, "Left", base)
	left.setAttr("m", method("Left.m", "left"))
	right := mkclass(t, "Right", base)
	right.setAttr("m", method("Right.m", "right"))
	dia := mkclass(t, "Diamond", left, right)

	inst := &instanceObject{cls: dia, attrs: newAttrs()}
	// super(Left, inst).m resolves to Right.m
	sup, err := NewSuper(left, inst)
	if err != nil {
		t.Fatalf("NewSuper: %v", err)
	}
	got, err := superCallMethod(sup.(*superObject), "m", nil)
	if err != nil {
		t.Fatalf("superCallMethod: %v", err)
	}
	if Repr(got) != "'right'" {
		t.Errorf("super(Left) reached %s, want 'right'", Repr(got))
	}
}

func TestSuperBindsOriginalSelf(t *testing.T) {
	base := mkclass(t, "Base")
	base.setAttr("m", method("Base.m", "base"))
	derived := mkclass(t, "Derived", base)
	inst := &instanceObject{cls: derived, attrs: newAttrs()}
	sup, _ := NewSuper(derived, inst)
	got, err := superLoadAttr(sup.(*superObject), "m")
	if err != nil {
		t.Fatalf("superLoadAttr: %v", err)
	}
	bm, ok := got.(*boundMethod)
	if !ok || bm.self != inst {
		t.Fatalf("super().m did not bind the original instance, got %T", got)
	}
}

func TestSuperArg1NotType(t *testing.T) {
	_, err := NewSuper(NewInt(1), NewInt(2))
	if err == nil || !strings.Contains(err.Error(), "super() argument 1 must be a type, not int") {
		t.Fatalf("error = %v, want argument-1 type message", err)
	}
}

func TestSuperArg2NotInstance(t *testing.T) {
	b := mkclass(t, "B")
	_, err := NewSuper(b, NewInt(5))
	if err == nil || !strings.Contains(err.Error(),
		"super(type, obj): obj (instance of int) is not an instance or subtype of type (B).") {
		t.Fatalf("error = %v, want argument-2 message", err)
	}
}

func TestSuperUnrelatedClass(t *testing.T) {
	a := mkclass(t, "A")
	u := mkclass(t, "U")
	inst := &instanceObject{cls: u, attrs: newAttrs()}
	_, err := NewSuper(a, inst)
	if err == nil || !strings.Contains(err.Error(), "instance of U") {
		t.Fatalf("error = %v, want unrelated-class rejection", err)
	}
}

func TestSuperMissingAttr(t *testing.T) {
	a := mkclass(t, "A")
	d := mkclass(t, "D", a)
	inst := &instanceObject{cls: d, attrs: newAttrs()}
	sup, _ := NewSuper(d, inst)
	_, err := superLoadAttr(sup.(*superObject), "nope")
	if err == nil || !strings.Contains(err.Error(), "'super' object has no attribute 'nope'") {
		t.Fatalf("error = %v, want super AttributeError", err)
	}
}

func TestSuperRepr(t *testing.T) {
	a := mkclass(t, "A")
	w := mkclass(t, "W", a)
	inst := &instanceObject{cls: w, attrs: newAttrs()}
	sup, _ := NewSuper(w, inst)
	if got := superRepr(sup.(*superObject)); got != "<super: <class 'W'>, <W object>>" {
		t.Errorf("superRepr = %q", got)
	}
}
