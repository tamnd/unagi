package objects

import "testing"

// TestSeqRepeatByIntSubclass checks that the * operator repeats a sequence when
// the multiplier is an int value subclass, the lossless __index__ coercion
// CPython's sq_repeat gives an int subclass, in either operand order.
func TestSeqRepeatByIntSubclass(t *testing.T) {
	c := buildIntSubclass(t, "MyInt", nil, nil)
	three := mustInstance(t, c, 3)

	got, err := Mul(NewStr("ab"), three)
	if err != nil || Str(got) != "ababab" {
		t.Fatalf(`"ab" * MyInt(3) = %v, %v; want ababab`, got, err)
	}
	got, err = Mul(three, NewStr("ab"))
	if err != nil || Str(got) != "ababab" {
		t.Fatalf(`MyInt(3) * "ab" = %v, %v; want ababab`, got, err)
	}

	got, err = Mul(NewList([]Object{NewInt(1)}), three)
	if err != nil || Repr(got) != "[1, 1, 1]" {
		t.Fatalf("[1] * MyInt(3) = %v, %v; want [1, 1, 1]", got, err)
	}
}

// TestSeqRepeatNegativeClamps checks that a negative or zero int-subclass count
// yields the empty sequence rather than an error, the way CPython clamps.
func TestSeqRepeatNegativeClamps(t *testing.T) {
	c := buildIntSubclass(t, "MyInt", nil, nil)
	neg := mustInstance(t, c, -4)

	got, err := Mul(NewStr("ab"), neg)
	if err != nil || Str(got) != "" {
		t.Fatalf(`"ab" * MyInt(-4) = %q, %v; want empty`, Str(got), err)
	}
}

// TestBytesCountByIntSubclass checks that the bytes constructor reads an int
// subclass as a count, building that many zero bytes, the way CPython's
// PyIndex_Check gate does.
func TestBytesCountByIntSubclass(t *testing.T) {
	c := buildIntSubclass(t, "MyInt", nil, nil)
	two := mustInstance(t, c, 2)

	b, err := bytesFromSource(two, "bytes", "bytes must be in range(0, 256)")
	if err != nil {
		t.Fatalf("bytes(MyInt(2)): %v", err)
	}
	if len(b) != 2 || b[0] != 0 || b[1] != 0 {
		t.Fatalf("bytes(MyInt(2)) = %v; want two zero bytes", b)
	}
}
