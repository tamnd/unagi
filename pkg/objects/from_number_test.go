package objects

import "testing"

// TestFloatFromNumber checks float.from_number coerces a real number and rejects
// a string, the way the Python 3.14 classmethod does where the float()
// constructor would parse the string.
func TestFloatFromNumber(t *testing.T) {
	fn, ok := builtinTypeClassmethod("float", "from_number")
	if !ok {
		t.Fatal("float.from_number not resolved")
	}
	// A float, int and bool each convert to the float value.
	for _, tt := range []struct {
		in   Object
		want float64
	}{
		{NewFloat(2.5), 2.5},
		{NewInt(5), 5.0},
		{True, 1.0},
	} {
		got, err := CallKw(fn, []Object{tt.in}, nil, nil)
		if err != nil {
			t.Fatalf("from_number(%s): %v", tt.in.TypeName(), err)
		}
		if v, okf := AsFloat(got); !okf || v != tt.want {
			t.Fatalf("from_number(%s) = %v; want %v", tt.in.TypeName(), got, tt.want)
		}
	}
	// A str is not a real number, so it is rejected rather than parsed.
	_, err := CallKw(fn, []Object{NewStr("2.5")}, nil, nil)
	if err == nil {
		t.Fatal("from_number(str) did not raise")
	}
	if msg := err.Error(); msg != "TypeError: must be real number, not str" {
		t.Fatalf("from_number(str) message = %q; want must be real number, not str", msg)
	}
	// The arity and keyword errors match CPython word for word.
	if _, err := CallKw(fn, nil, nil, nil); err == nil ||
		err.Error() != "TypeError: float.from_number() takes exactly one argument (0 given)" {
		t.Fatalf("from_number() arity error = %v", err)
	}
	if _, err := CallKw(fn, []Object{NewInt(1)}, []string{"number"}, []Object{NewInt(1)}); err == nil ||
		err.Error() != "TypeError: float.from_number() takes no keyword arguments" {
		t.Fatalf("from_number(kw) error = %v", err)
	}
}

// TestComplexFromNumber checks complex.from_number coerces a number, including a
// complex, and rejects a string.
func TestComplexFromNumber(t *testing.T) {
	fn, ok := builtinTypeClassmethod("complex", "from_number")
	if !ok {
		t.Fatal("complex.from_number not resolved")
	}
	got, err := CallKw(fn, []Object{NewComplex(1, 2)}, nil, nil)
	if err != nil {
		t.Fatalf("from_number(complex): %v", err)
	}
	if re, im, okc := asComplex(got); !okc || re != 1 || im != 2 {
		t.Fatalf("from_number(1+2j) = %v; want (1+2j)", got)
	}
	// An int reads as a real with a zero imaginary part.
	got, err = CallKw(fn, []Object{NewInt(3)}, nil, nil)
	if err != nil {
		t.Fatalf("from_number(int): %v", err)
	}
	if re, im, _ := asComplex(got); re != 3 || im != 0 {
		t.Fatalf("from_number(3) = %v; want (3+0j)", got)
	}
	// A str is rejected with the same real-number message.
	if _, err := CallKw(fn, []Object{NewStr("1")}, nil, nil); err == nil ||
		err.Error() != "TypeError: must be real number, not str" {
		t.Fatalf("from_number(str) error = %v", err)
	}
}

// TestFromNumberOnlyFloatAndComplex checks int gained no from_number classmethod
// in Python 3.14, so it declines the name while float and complex answer it.
func TestFromNumberOnlyFloatAndComplex(t *testing.T) {
	if _, ok := builtinTypeClassmethod("int", "from_number"); ok {
		t.Fatal("int.from_number should not resolve")
	}
	if _, ok := builtinTypeClassmethod("float", "from_number"); !ok {
		t.Fatal("float.from_number should resolve")
	}
	if _, ok := builtinTypeClassmethod("complex", "from_number"); !ok {
		t.Fatal("complex.from_number should resolve")
	}
}

// TestFromNumberSubclassRebuilds checks a float subclass inherits from_number and
// rebuilds the subclass, the way CPython constructs cls.
func TestFromNumberSubclassRebuilds(t *testing.T) {
	c := buildFloatSubclass(t, "MyFloat")
	fn, ok := valueSubclassClassmethod(c, "float", "from_number")
	if !ok {
		t.Fatal("MyFloat.from_number not resolved")
	}
	got, err := CallKw(fn, []Object{NewInt(3)}, nil, nil)
	if err != nil {
		t.Fatalf("MyFloat.from_number: %v", err)
	}
	inst, ok := got.(*instanceObject)
	if !ok || inst.cls != c {
		t.Fatalf("from_number returned %T (%s); want a MyFloat instance", got, got.TypeName())
	}
	if v, _ := AsFloat(got); v != 3.0 {
		t.Fatalf("MyFloat.from_number(3) = %v; want 3.0", got)
	}
}
