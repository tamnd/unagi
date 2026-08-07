package objects

import "testing"

// TestBuiltinClassmethodSelfAndQualname checks a classmethod read off a builtin
// type constructor carries __self__ (the type) and a qualified __qualname__ the
// way CPython's does, while __name__ stays the bare method name.
func TestBuiltinClassmethodSelfAndQualname(t *testing.T) {
	for _, tc := range []struct{ typeName, method string }{
		{"int", "from_bytes"},
		{"float", "fromhex"},
		{"float", "from_number"},
		{"complex", "from_number"},
		{"dict", "fromkeys"},
		{"bytes", "fromhex"},
	} {
		ctor := builtinTypeReprCtor(t, tc.typeName)
		cm, err := LoadAttr(ctor, tc.method)
		if err != nil {
			t.Fatalf("%s.%s: %v", tc.typeName, tc.method, err)
		}
		self, err := LoadAttr(cm, "__self__")
		if err != nil {
			t.Fatalf("%s.%s.__self__: %v", tc.typeName, tc.method, err)
		}
		if self != ctor {
			t.Fatalf("%s.%s.__self__ = %v; want the type constructor", tc.typeName, tc.method, self)
		}
		wantQual := tc.typeName + "." + tc.method
		if got := loadStr(t, cm, "__qualname__"); got != wantQual {
			t.Fatalf("%s.%s.__qualname__ = %q; want %q", tc.typeName, tc.method, got, wantQual)
		}
		if got := loadStr(t, cm, "__name__"); got != tc.method {
			t.Fatalf("%s.%s.__name__ = %q; want %q", tc.typeName, tc.method, got, tc.method)
		}
	}
}

// TestBuiltinStaticmethodSelfIsNone checks the maketrans staticmethods report
// __self__ = None rather than the type, matching CPython, while still carrying a
// qualified __qualname__.
func TestBuiltinStaticmethodSelfIsNone(t *testing.T) {
	for _, typeName := range []string{"str", "bytes"} {
		ctor := builtinTypeReprCtor(t, typeName)
		cm, err := LoadAttr(ctor, "maketrans")
		if err != nil {
			t.Fatalf("%s.maketrans: %v", typeName, err)
		}
		self, err := LoadAttr(cm, "__self__")
		if err != nil {
			t.Fatalf("%s.maketrans.__self__: %v", typeName, err)
		}
		if self != None {
			t.Fatalf("%s.maketrans.__self__ = %v; want None", typeName, self)
		}
		if got := loadStr(t, cm, "__qualname__"); got != typeName+".maketrans" {
			t.Fatalf("%s.maketrans.__qualname__ = %q", typeName, got)
		}
	}
}

// TestOrdinaryBuiltinHasNoBoundSelf guards the boundary: a plain builtin
// function is not stamped, so reading __self__ off it still reports the miss.
func TestOrdinaryBuiltinHasNoBoundSelf(t *testing.T) {
	fn := NewFunc("len", -1, func(args []Object) (Object, error) { return None, nil })
	if _, err := LoadAttr(fn, "__self__"); !isKind(err, AttributeError) {
		t.Fatalf("len.__self__ = %v; want AttributeError", err)
	}
}

func builtinTypeReprCtor(t *testing.T, name string) *funcObject {
	t.Helper()
	if !builtinTypeReprs[name] {
		t.Fatalf("%s is not a registered builtin type name", name)
	}
	return &funcObject{name: name, arity: -1, fn: func(args []Object) (Object, error) { return None, nil }}
}

func loadStr(t *testing.T, o Object, name string) string {
	t.Helper()
	v, err := LoadAttr(o, name)
	if err != nil {
		t.Fatalf("LoadAttr(%s): %v", name, err)
	}
	s, ok := v.(*strObject)
	if !ok {
		t.Fatalf("%s is %T, want str", name, v)
	}
	return s.v
}
