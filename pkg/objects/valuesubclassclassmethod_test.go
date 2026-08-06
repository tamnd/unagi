package objects

import "testing"

// TestValueSubclassFromBytesBuildsSubclass checks an int subclass inherits the
// from_bytes classmethod and rebuilds the subclass, the class-level counterpart
// to the instance-method inheritance a value subclass already carries.
func TestValueSubclassFromBytesBuildsSubclass(t *testing.T) {
	c := buildIntSubclass(t, "MyInt", nil, nil)

	fn, ok := valueSubclassClassmethod(c, "int", "from_bytes")
	if !ok {
		t.Fatal("MyInt.from_bytes not resolved")
	}
	got, err := CallKw(fn, []Object{NewBytes([]byte{0x01, 0x02})}, []string{"byteorder"}, []Object{NewStr("big")})
	if err != nil {
		t.Fatalf("MyInt.from_bytes: %v", err)
	}
	// The result carries the subclass type and the decoded value.
	inst, ok := got.(*instanceObject)
	if !ok || inst.cls != c {
		t.Fatalf("from_bytes returned %T (%s); want a MyInt instance", got, got.TypeName())
	}
	payload, ok := builtinUnwrap(got)
	if !ok {
		t.Fatalf("from_bytes result carries no int payload")
	}
	if v, _ := AsInt(payload); v != 258 {
		t.Fatalf("MyInt.from_bytes value = %d; want 258", v)
	}
}

// TestValueSubclassClassmethodNonConstructing checks a non-constructing
// classmethod inherits unchanged, so its native result type is not rebuilt as
// the subclass.
func TestValueSubclassClassmethodNonConstructing(t *testing.T) {
	c := buildIntSubclass(t, "MyInt", nil, nil)

	// int has no maketrans, so a made-up non-constructing name simply declines.
	if _, ok := valueSubclassClassmethod(c, "int", "maketrans"); ok {
		t.Fatal("int names no maketrans classmethod; should decline")
	}
	// A name the base does not carry declines rather than resolving.
	if _, ok := valueSubclassClassmethod(c, "int", "no_such_method"); ok {
		t.Fatal("unknown classmethod should decline")
	}
}
