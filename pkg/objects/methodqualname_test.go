package objects

import "testing"

// TestBoundMethodQualnameNamesReceiverType checks a bound builtin method names
// __qualname__ after the type it was read off, so a list method reads list.name
// and a numeric method reads its own type, while __name__ stays bare. CPython
// names a bound method after the receiver's own type, so True.bit_length reads
// bool.bit_length rather than the defining type.
func TestBoundMethodQualnameNamesReceiverType(t *testing.T) {
	cases := []struct {
		recv     Object
		name     string
		qualname string
	}{
		{NewList(nil), "append", "list.append"},
		{NewStr("ab"), "upper", "str.upper"},
		{NewInt(255), "to_bytes", "int.to_bytes"},
		{NewInt(255), "bit_length", "int.bit_length"},
		{True, "bit_length", "bool.bit_length"},
		{NewFloat(1.5), "is_integer", "float.is_integer"},
	}
	for _, c := range cases {
		m, err := LoadAttr(c.recv, c.name)
		if err != nil {
			t.Fatalf("%s.%s: %v", c.recv.TypeName(), c.name, err)
		}
		if got := loadStr(t, m, "__qualname__"); got != c.qualname {
			t.Fatalf("%s.%s.__qualname__ = %q; want %q", c.recv.TypeName(), c.name, got, c.qualname)
		}
		if got := loadStr(t, m, "__name__"); got != c.name {
			t.Fatalf("%s.%s.__name__ = %q; want %q", c.recv.TypeName(), c.name, got, c.name)
		}
	}
}

// TestBoundNumericDunderQualname checks a numeric operator dunder read off an
// instance names __qualname__ after the receiver's type, so (5).__add__ reads
// int.__add__ and True.__and__ reads bool.__and__.
func TestBoundNumericDunderQualname(t *testing.T) {
	cases := []struct {
		recv     Object
		name     string
		qualname string
	}{
		{NewInt(5), "__add__", "int.__add__"},
		{NewInt(5), "__neg__", "int.__neg__"},
		{True, "__and__", "bool.__and__"},
	}
	for _, c := range cases {
		m, err := LoadAttr(c.recv, c.name)
		if err != nil {
			t.Fatalf("%s.%s: %v", c.recv.TypeName(), c.name, err)
		}
		if got := loadStr(t, m, "__qualname__"); got != c.qualname {
			t.Fatalf("%s.%s.__qualname__ = %q; want %q", c.recv.TypeName(), c.name, got, c.qualname)
		}
	}
}
