package objects

import "testing"

// TestBoundInstanceMethodSelf checks a bound builtin instance method reports its
// receiver through __self__, the way CPython's bound method does, across the
// central builtinMethodValue path and the numeric inline paths.
func TestBoundInstanceMethodSelf(t *testing.T) {
	recv := NewList([]Object{NewInt(1)})
	m := builtinMethodValue(recv, "append")
	self, err := LoadAttr(m, "__self__")
	if err != nil {
		t.Fatalf("append.__self__: %v", err)
	}
	if self != recv {
		t.Fatalf("append.__self__ = %v; want the receiver", self)
	}
}

// TestBoundNumericMethodSelf checks the int and float inline method paths stamp
// the receiver, so (255).to_bytes.__self__ and (1.5).is_integer.__self__ read
// back their number.
func TestBoundNumericMethodSelf(t *testing.T) {
	i := NewInt(255)
	for _, name := range []string{"to_bytes", "bit_length", "conjugate"} {
		m, err := LoadAttr(i, name)
		if err != nil {
			t.Fatalf("int.%s: %v", name, err)
		}
		self, err := LoadAttr(m, "__self__")
		if err != nil {
			t.Fatalf("int.%s.__self__: %v", name, err)
		}
		if self != i {
			t.Fatalf("int.%s.__self__ = %v; want the receiver", name, self)
		}
	}
	f := NewFloat(1.5)
	m, err := LoadAttr(f, "is_integer")
	if err != nil {
		t.Fatalf("float.is_integer: %v", err)
	}
	self, err := LoadAttr(m, "__self__")
	if err != nil {
		t.Fatalf("float.is_integer.__self__: %v", err)
	}
	if self != f {
		t.Fatalf("float.is_integer.__self__ = %v; want the receiver", self)
	}
}

// TestBoundInstanceMethodNameStaysBare guards the boundary: stamping __self__
// leaves __name__ the bare method name.
func TestBoundInstanceMethodNameStaysBare(t *testing.T) {
	m := builtinMethodValue(NewList(nil), "append")
	if got := loadStr(t, m, "__name__"); got != "append" {
		t.Fatalf("append.__name__ = %q; want %q", got, "append")
	}
}
