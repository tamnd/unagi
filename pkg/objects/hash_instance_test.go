package objects

import (
	"strings"
	"testing"
)

// A class that overrides neither __eq__ nor __hash__ hashes by identity, so an
// instance is a usable dict key and two instances stay distinct.
func TestInstanceHashIdentity(t *testing.T) {
	c := mkclass(t, "C")
	x, y := inst(c), inst(c)
	hx, err := PyHash(x)
	if err != nil {
		t.Fatalf("PyHash(x): %v", err)
	}
	if hx2, _ := PyHash(x); hx2 != hx {
		t.Fatalf("PyHash not stable within a run: %d vs %d", hx, hx2)
	}
	d := &dictObject{index: map[string]int{}}
	if err := d.set(x, NewInt(1)); err != nil {
		t.Fatalf("set x: %v", err)
	}
	if err := d.set(y, NewInt(2)); err != nil {
		t.Fatalf("set y: %v", err)
	}
	if len(d.entries) != 2 {
		t.Fatalf("distinct instances collided: %d entries", len(d.entries))
	}
	got, err := d.get(x)
	if err != nil || !equals(got, NewInt(1)) {
		t.Fatalf("d[x] = %v, %v, want 1", got, err)
	}
}

// A class that defines __eq__ without __hash__ is unhashable, and the dict and
// set boundaries wrap the TypeError the way CPython 3.14 reports it.
func TestInstanceHashEqOnlyUnhashable(t *testing.T) {
	c := mkclass(t, "C")
	c.setAttr("__eq__", mkfn("C.__eq__", 2, func(args []Object) (Object, error) {
		return True, nil
	}))
	x := inst(c)
	if _, err := PyHash(x); err == nil ||
		!strings.Contains(err.(*Exception).Text(), "unhashable type: 'C'") {
		t.Fatalf("PyHash unhashable error = %v", err)
	}
	d := &dictObject{index: map[string]int{}}
	if err := d.set(x, NewInt(1)); err == nil ||
		!strings.Contains(err.(*Exception).Text(), "cannot use 'C' as a dict key (unhashable type: 'C')") {
		t.Fatalf("dict-key error = %v", err)
	}
	if _, err := setKey(x); err == nil ||
		!strings.Contains(err.(*Exception).Text(), "cannot use 'C' as a set element (unhashable type: 'C')") {
		t.Fatalf("set-element error = %v", err)
	}
}

// An explicit __hash__ = None on the class makes the instance unhashable even
// without a user __eq__.
func TestInstanceHashNoneUnhashable(t *testing.T) {
	c := mkclass(t, "C")
	c.setAttr("__hash__", None)
	if _, err := PyHash(inst(c)); err == nil ||
		!strings.Contains(err.(*Exception).Text(), "unhashable type: 'C'") {
		t.Fatalf("PyHash None-hash error = %v", err)
	}
}

// A user __hash__ decides the value, mapping -1 to -2 like CPython and raising
// on a non-integer result.
func TestInstanceHashCustom(t *testing.T) {
	c := mkclass(t, "C")
	c.setAttr("__hash__", mkfn("C.__hash__", 1, func(args []Object) (Object, error) {
		return NewInt(42), nil
	}))
	if h, err := PyHash(inst(c)); err != nil || h != 42 {
		t.Fatalf("PyHash = %d, %v, want 42", h, err)
	}

	neg := mkclass(t, "Neg")
	neg.setAttr("__hash__", mkfn("Neg.__hash__", 1, func(args []Object) (Object, error) {
		return NewInt(-1), nil
	}))
	if h, err := PyHash(inst(neg)); err != nil || h != -2 {
		t.Fatalf("PyHash(-1) = %d, %v, want -2", h, err)
	}

	bad := mkclass(t, "Bad")
	bad.setAttr("__hash__", mkfn("Bad.__hash__", 1, func(args []Object) (Object, error) {
		return NewStr("x"), nil
	}))
	if _, err := PyHash(inst(bad)); err == nil ||
		!strings.Contains(err.(*Exception).Text(), "__hash__ method should return an integer") {
		t.Fatalf("non-int __hash__ error = %v", err)
	}
}

// A class that defines both __eq__ and __hash__ keys the dict by the hash value,
// so two equal instances that hash the same land on one slot.
func TestInstanceHashEqValueKeying(t *testing.T) {
	c := mkclass(t, "C")
	c.setAttr("__eq__", mkfn("C.__eq__", 2, func(args []Object) (Object, error) {
		return True, nil
	}))
	c.setAttr("__hash__", mkfn("C.__hash__", 1, func(args []Object) (Object, error) {
		return NewInt(7), nil
	}))
	d := &dictObject{index: map[string]int{}}
	if err := d.set(inst(c), NewInt(1)); err != nil {
		t.Fatalf("set first: %v", err)
	}
	if err := d.set(inst(c), NewInt(2)); err != nil {
		t.Fatalf("set second: %v", err)
	}
	if len(d.entries) != 1 {
		t.Fatalf("equal instances did not collide: %d entries", len(d.entries))
	}
	got, err := d.get(inst(c))
	if err != nil || !equals(got, NewInt(2)) {
		t.Fatalf("d[C()] = %v, %v, want 2", got, err)
	}
}
