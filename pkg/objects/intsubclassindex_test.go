package objects

import "testing"

// TestIndexOfUnwrapsIntSubclass checks that IndexOf resolves an int value
// subclass to its stored value, the lossless coercion CPython's PyNumber_Index
// gives an int subclass, so range, chr, subscription and the integer math
// routines all accept one.
func TestIndexOfUnwrapsIntSubclass(t *testing.T) {
	c := buildIntSubclass(t, "MyInt", nil, nil)
	a := mustInstance(t, c, 5)

	got, ok, err := IndexOf(a)
	if err != nil || !ok {
		t.Fatalf("IndexOf(MyInt(5)) = %v, %v, %v; want a value, true, nil", got, ok, err)
	}
	if v, isInt := AsInt(got); !isInt || v != 5 {
		t.Fatalf("IndexOf(MyInt(5)) value = %v; want 5", got)
	}
	if got.TypeName() != "int" {
		t.Fatalf("IndexOf(MyInt(5)) type = %q; want int", got.TypeName())
	}
}

// TestIndexOfLeavesFloatSubclass checks that a float subclass, which has no
// __index__, is left unhandled so an integer-required caller keeps its own error.
func TestIndexOfLeavesFloatSubclass(t *testing.T) {
	c := buildFloatSubclass(t, "MyFloat")
	a, err := Instantiate(c, []Object{NewFloat(3.0)}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate MyFloat(3.0): %v", err)
	}
	got, ok, err := IndexOf(a)
	if err != nil {
		t.Fatalf("IndexOf(MyFloat(3.0)) err = %v; want nil", err)
	}
	if ok {
		t.Fatalf("IndexOf(MyFloat(3.0)) = %v, ok; want not handled", got)
	}
}
