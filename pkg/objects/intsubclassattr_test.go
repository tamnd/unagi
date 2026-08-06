package objects

import "testing"

// TestIntSubclassInheritsMethods checks that an int value subclass answers the
// named int methods and the rational-view data attributes off its payload, the
// inheritance CPython gives an int subclass, so bit_length(), as_integer_ratio()
// and numerator all resolve when the class defines no override.
func TestIntSubclassInheritsMethods(t *testing.T) {
	c := buildIntSubclass(t, "MyInt", nil, nil)
	a := mustInstance(t, c, 12)

	bl, err := LoadAttr(a, "bit_length")
	if err != nil {
		t.Fatalf("MyInt(12).bit_length attr: %v", err)
	}
	got, err := Call(bl, nil)
	if err != nil || Str(got) != "4" {
		t.Fatalf("MyInt(12).bit_length() = %v, %v; want 4", got, err)
	}

	ratio, err := LoadAttr(a, "as_integer_ratio")
	if err != nil {
		t.Fatalf("MyInt(12).as_integer_ratio attr: %v", err)
	}
	got, err = Call(ratio, nil)
	if err != nil || Repr(got) != "(12, 1)" {
		t.Fatalf("MyInt(12).as_integer_ratio() = %v, %v; want (12, 1)", got, err)
	}
}

// TestIntSubclassInheritsDataAttrs checks that the read-only rational view
// (numerator, denominator, real, imag) reads off the payload and comes back as a
// plain int, the way CPython's int subclass answers them.
func TestIntSubclassInheritsDataAttrs(t *testing.T) {
	c := buildIntSubclass(t, "MyInt", nil, nil)
	a := mustInstance(t, c, 12)

	for _, tc := range []struct {
		name string
		want string
	}{
		{"numerator", "12"},
		{"denominator", "1"},
		{"real", "12"},
		{"imag", "0"},
	} {
		got, err := LoadAttr(a, tc.name)
		if err != nil {
			t.Fatalf("MyInt(12).%s: %v", tc.name, err)
		}
		if Str(got) != tc.want {
			t.Fatalf("MyInt(12).%s = %v; want %s", tc.name, got, tc.want)
		}
		if got.TypeName() != "int" {
			t.Fatalf("MyInt(12).%s type = %q; want int", tc.name, got.TypeName())
		}
	}
}

// TestIntSubclassMethodOverrideWins checks that a class-level override still
// shadows the inherited member, so the payload fallback never masks a user
// method, the same precedence the other value subclasses keep.
func TestIntSubclassMethodOverrideWins(t *testing.T) {
	over := NewFunc("bit_length", -1, func(args []Object) (Object, error) {
		return NewInt(999), nil
	})
	c := buildIntSubclass(t, "Over", []string{"bit_length"}, []Object{over})
	a := mustInstance(t, c, 5)

	fn, err := LoadAttr(a, "bit_length")
	if err != nil {
		t.Fatalf("Over(5).bit_length attr: %v", err)
	}
	got, err := Call(fn, nil)
	if err != nil || Str(got) != "999" {
		t.Fatalf("Over(5).bit_length() = %v, %v; want 999 from the override", got, err)
	}
}
