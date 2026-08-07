package objects

import "testing"

// TestBoundNumericDunderSelf checks a numeric operator or special dunder read off
// an int instance, (5).__add__, is a bound method carrying the receiver through
// __self__, the way CPython's method-wrapper does, across the binary, unary and
// special dunder paths numericBoundDunder serves.
func TestBoundNumericDunderSelf(t *testing.T) {
	i := NewInt(5)
	for _, name := range []string{"__add__", "__radd__", "__neg__", "__abs__", "__hash__", "__divmod__", "__round__", "__pow__"} {
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
}

// TestBoundNumericDunderNameStaysBare guards the boundary: stamping __self__
// leaves the dunder's __name__ the bare slot name.
func TestBoundNumericDunderNameStaysBare(t *testing.T) {
	m, err := LoadAttr(NewInt(5), "__add__")
	if err != nil {
		t.Fatalf("int.__add__: %v", err)
	}
	if got := loadStr(t, m, "__name__"); got != "__add__" {
		t.Fatalf("(5).__add__.__name__ = %q; want %q", got, "__add__")
	}
}
