package objects

import "testing"

// TestNumericSubclassMethodIdentity checks that a method read off a numeric value
// subclass instance binds to the instance, so __self__ is the instance and
// __qualname__ names the subclass (MyInt.bit_length), while a data attribute
// (real, imag, numerator) still reads straight through as a plain value. Before
// the rebind, numericSubclassAttr handed back the payload-bound method whose
// __self__ was the plain int and whose __qualname__ was int.bit_length.
func TestNumericSubclassMethodIdentity(t *testing.T) {
	c := buildIntSubclass(t, "MyInt", nil, nil)
	inst, err := Instantiate(c, []Object{NewInt(12)}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	m, err := LoadAttr(inst, "bit_length")
	if err != nil {
		t.Fatalf("load bit_length: %v", err)
	}
	self, err := LoadAttr(m, "__self__")
	if err != nil {
		t.Fatalf("__self__: %v", err)
	}
	if self != Object(inst) {
		t.Fatalf("__self__ = %v; want the instance", self)
	}
	qn, err := LoadAttr(m, "__qualname__")
	if err != nil {
		t.Fatalf("__qualname__: %v", err)
	}
	if s := Str(qn); s != "MyInt.bit_length" {
		t.Fatalf("__qualname__ = %q; want MyInt.bit_length", s)
	}

	// A data attribute reads straight through as a plain int, not a bound method.
	num, err := LoadAttr(inst, "numerator")
	if err != nil {
		t.Fatalf("load numerator: %v", err)
	}
	if _, ok := num.(*funcObject); ok {
		t.Fatalf("numerator resolved to a method; want a plain value")
	}
	if v, _ := AsInt(num); v != 12 {
		t.Fatalf("numerator = %d; want 12", v)
	}
}
