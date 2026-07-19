package objects

import "testing"

// weakrefClass builds a bare `class Name: pass`, whose instances carry weak
// reference support the way a user class does.
func weakrefClass(t *testing.T) *classObject {
	t.Helper()
	c, err := buildClass(nil, "C", "__main__.C", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("build C: %v", err)
	}
	cc, ok := c.(*classObject)
	if !ok {
		t.Fatalf("build C: not a class")
	}
	return cc
}

func TestWeakrefCallReturnsReferent(t *testing.T) {
	c := weakrefClass(t)
	inst, err := Instantiate(c, nil, nil, nil)
	if err != nil {
		t.Fatalf("instantiate C: %v", err)
	}
	r, err := NewWeakref(inst, nil)
	if err != nil {
		t.Fatalf("NewWeakref: %v", err)
	}
	got, err := Call(r, nil)
	if err != nil {
		t.Fatalf("call ref: %v", err)
	}
	if got != inst {
		t.Fatalf("ref() = %v, want the referent", got)
	}
	if !Callable(r) {
		t.Fatalf("Callable(ref) = false, want true")
	}
	if _, err := Call(r, []Object{inst}); err == nil {
		t.Fatalf("ref(arg) did not raise, want TypeError")
	}
}

func TestWeakrefHashAndEqualByReferent(t *testing.T) {
	c := weakrefClass(t)
	inst, _ := Instantiate(c, nil, nil, nil)
	r1, _ := NewWeakref(inst, nil)
	r2, _ := NewWeakref(inst, nil)

	h1, err := PyHash(r1)
	if err != nil {
		t.Fatalf("hash r1: %v", err)
	}
	h2, _ := PyHash(r2)
	hi, _ := PyHash(inst)
	if h1 != h2 || h1 != hi {
		t.Fatalf("hashes = %d, %d, %d; want all equal to the referent hash", h1, h2, hi)
	}
	if !equals(r1, r2) {
		t.Fatalf("two refs to one object compare unequal")
	}
	if equals(r1, inst) {
		t.Fatalf("a ref compares equal to its bare referent")
	}

	// Two refs to one object share a set slot; a ref never collides with the
	// bare object.
	set, err := NewSet([]Object{r1, r2, inst})
	if err != nil {
		t.Fatalf("build set: %v", err)
	}
	n, err := Len(set)
	if err != nil {
		t.Fatalf("len set: %v", err)
	}
	if n != 2 {
		t.Fatalf("set of {ref, ref, obj} has %d elements, want 2", n)
	}
}

func TestWeakrefRejectsUnreferenceable(t *testing.T) {
	if _, err := NewWeakref(NewInt(5), nil); err == nil {
		t.Fatalf("NewWeakref(int) did not raise, want TypeError")
	}
	if _, err := NewWeakref(NewStr("x"), nil); err == nil {
		t.Fatalf("NewWeakref(str) did not raise, want TypeError")
	}
}
