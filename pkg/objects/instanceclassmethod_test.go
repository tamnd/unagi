package objects

import "testing"

// TestInstanceClassmethodQualname checks a classmethod resolved off an instance
// qualifies __qualname__ to type.name while __name__ stays bare, the way CPython
// inherits the classmethod onto an instance. The __self__ = type half needs the
// runtime's BuiltinTypeResolver and is covered end-to-end by the fixture.
func TestInstanceClassmethodQualname(t *testing.T) {
	cases := []struct{ typeName, name, qualname string }{
		{"int", "from_bytes", "int.from_bytes"},
		{"bool", "from_bytes", "bool.from_bytes"},
		{"float", "fromhex", "float.fromhex"},
		{"complex", "from_number", "complex.from_number"},
		{"bytes", "fromhex", "bytes.fromhex"},
		{"bytearray", "fromhex", "bytearray.fromhex"},
	}
	for _, c := range cases {
		v, ok := builtinInstanceClassmethod(c.typeName, c.name)
		if !ok {
			t.Fatalf("builtinInstanceClassmethod(%q, %q) not found", c.typeName, c.name)
		}
		if got := loadStr(t, v, "__qualname__"); got != c.qualname {
			t.Fatalf("%s.%s.__qualname__ = %q; want %q", c.typeName, c.name, got, c.qualname)
		}
		if got := loadStr(t, v, "__name__"); got != c.name {
			t.Fatalf("%s.%s.__name__ = %q; want %q", c.typeName, c.name, got, c.name)
		}
	}
}

// TestInstanceStaticmethodSelfIsNone checks a staticmethod resolved off an
// instance, "abc".maketrans, reports __self__ = None the way CPython does, and
// still qualifies __qualname__.
func TestInstanceStaticmethodSelfIsNone(t *testing.T) {
	v, ok := builtinInstanceClassmethod("str", "maketrans")
	if !ok {
		t.Fatal("str.maketrans not found")
	}
	self, err := LoadAttr(v, "__self__")
	if err != nil {
		t.Fatalf("str.maketrans.__self__: %v", err)
	}
	if self != None {
		t.Fatalf("str.maketrans.__self__ = %v; want None", self)
	}
	if got := loadStr(t, v, "__qualname__"); got != "str.maketrans" {
		t.Fatalf("str.maketrans.__qualname__ = %q; want %q", got, "str.maketrans")
	}
}

// TestInstanceClassmethodMissIsNotFound guards the boundary: a name that is not a
// class-level method of the type resolves to no classmethod, so the caller keeps
// its own AttributeError rather than fabricating one.
func TestInstanceClassmethodMissIsNotFound(t *testing.T) {
	if _, ok := builtinInstanceClassmethod("list", "nope"); ok {
		t.Fatal("list.nope resolved a classmethod; want miss")
	}
	if _, ok := builtinInstanceClassmethod("int", "bit_length"); ok {
		t.Fatal("int.bit_length resolved as a classmethod; it is an instance method")
	}
}
