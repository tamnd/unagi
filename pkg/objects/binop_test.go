package objects

import "testing"

// dunder builds a plain two-parameter method (self, other) returning the given
// closure's result, so a test class can carry an operator dunder.
func dunder(qual string, impl func(self, other Object) (Object, error)) *functionObject {
	return NewFunction(qual, []Param{
		{Name: "self", Kind: ParamPlain},
		{Name: "other", Kind: ParamPlain},
	}, nil, func(args []Object) (Object, error) {
		return impl(args[0], args[1])
	}).(*functionObject)
}

func TestNotImplementedSingleton(t *testing.T) {
	if got := Repr(NotImplemented); got != "NotImplemented" {
		t.Errorf("Repr(NotImplemented) = %q, want NotImplemented", got)
	}
	if got := NotImplemented.TypeName(); got != "NotImplementedType" {
		t.Errorf("TypeName = %q, want NotImplementedType", got)
	}
}

func TestMatMulUnsupportedInts(t *testing.T) {
	_, err := MatMul(NewInt(3), NewInt(4))
	want := "TypeError: unsupported operand type(s) for @: 'int' and 'int'"
	if err == nil || err.Error() != want {
		t.Errorf("MatMul(3, 4) err = %v, want %q", err, want)
	}
}

// The forward method handles a same-type pair and the reflected method is
// never consulted.
func TestBinaryDunderForward(t *testing.T) {
	c := mkclass(t, "V")
	c.setAttr("__add__", dunder("V.__add__", func(_, _ Object) (Object, error) {
		return NewStr("forward"), nil
	}))
	a := &instanceObject{cls: c, attrs: newAttrs()}
	b := &instanceObject{cls: c, attrs: newAttrs()}
	got, err := Add(a, b)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if Repr(got) != "'forward'" {
		t.Errorf("Add = %s, want 'forward'", Repr(got))
	}
}

// A plain-left, instance-right pair falls through the missing forward method
// to the right operand's reflected method.
func TestBinaryDunderReflectedFallback(t *testing.T) {
	c := mkclass(t, "R")
	c.setAttr("__radd__", dunder("R.__radd__", func(_, other Object) (Object, error) {
		return NewStr("radd:" + Repr(other)), nil
	}))
	b := &instanceObject{cls: c, attrs: newAttrs()}
	got, err := Add(NewInt(5), b)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if Repr(got) != "'radd:5'" {
		t.Errorf("Add = %s, want 'radd:5'", Repr(got))
	}
}

// When both operands decline with NotImplemented the operation raises the
// unsupported-operand TypeError rather than returning the sentinel.
func TestBinaryDunderNotImplementedRaises(t *testing.T) {
	c := mkclass(t, "D")
	c.setAttr("__add__", dunder("D.__add__", func(_, _ Object) (Object, error) {
		return NotImplemented, nil
	}))
	a := &instanceObject{cls: c, attrs: newAttrs()}
	b := &instanceObject{cls: c, attrs: newAttrs()}
	_, err := Add(a, b)
	want := "TypeError: unsupported operand type(s) for +: 'D' and 'D'"
	if err == nil || err.Error() != want {
		t.Errorf("Add err = %v, want %q", err, want)
	}
}

// A subclass that overrides the reflected method gets the first attempt over
// its base's forward method.
func TestBinaryDunderSubclassReflectedFirst(t *testing.T) {
	base := mkclass(t, "Base")
	base.setAttr("__sub__", dunder("Base.__sub__", func(_, _ Object) (Object, error) {
		return NewStr("base.sub"), nil
	}))
	base.setAttr("__rsub__", dunder("Base.__rsub__", func(_, _ Object) (Object, error) {
		return NewStr("base.rsub"), nil
	}))
	sub := mkclass(t, "Sub", base)
	sub.setAttr("__rsub__", dunder("Sub.__rsub__", func(_, _ Object) (Object, error) {
		return NewStr("sub.rsub"), nil
	}))
	a := &instanceObject{cls: base, attrs: newAttrs()}
	b := &instanceObject{cls: sub, attrs: newAttrs()}
	got, err := Sub(a, b)
	if err != nil {
		t.Fatalf("Sub: %v", err)
	}
	if Repr(got) != "'sub.rsub'" {
		t.Errorf("Sub = %s, want 'sub.rsub'", Repr(got))
	}
}
