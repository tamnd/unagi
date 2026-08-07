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

// refBaseValue is the weakref.ref builtin as a class statement names it: a
// funcObject spelled "ref" that builds a weakref payload, the base a ref
// subclass (weakref.py's KeyedRef and WeakMethod) inherits from.
func refBaseValue() Object {
	return NewFunc("ref", -1, func(args []Object) (Object, error) {
		var callback Object
		if len(args) == 2 {
			callback = args[1]
		}
		return NewWeakref(args[0], callback)
	})
}

// TestRefSubclassIsCallable checks a weakref.ref subclass instance inherits the
// base ref's call, returning the referent, so weakref.py's KeyedRef (which
// importlib's _ModuleLock reads by calling) works.
func TestRefSubclassIsCallable(t *testing.T) {
	base := refBaseValue()
	if name, ok := builtinBaseName(base); !ok || name != "ref" {
		t.Fatalf("builtinBaseName(ref) = %q, %v; want ref, true", name, ok)
	}
	c, err := buildClass(nil, "KeyedRef", "__main__.KeyedRef", []Object{base}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("build KeyedRef: %v", err)
	}
	cc := c.(*classObject)
	if cc.builtinBase != "ref" {
		t.Fatalf("builtinBase = %q, want ref", cc.builtinBase)
	}

	referent := weakrefClass(t)
	obj, _ := Instantiate(referent, nil, nil, nil)
	k, err := Instantiate(cc, []Object{obj}, nil, nil)
	if err != nil {
		t.Fatalf("instantiate KeyedRef: %v", err)
	}
	if !Callable(k) {
		t.Fatalf("Callable(KeyedRef instance) = false, want true")
	}
	got, err := Call(k, nil)
	if err != nil {
		t.Fatalf("call KeyedRef instance: %v", err)
	}
	if got != obj {
		t.Fatalf("KeyedRef() = %v, want the referent", got)
	}
	if _, err := Call(k, []Object{obj}); err == nil {
		t.Fatalf("KeyedRef(arg) did not raise, want TypeError")
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

// TestWeakrefArrayAndMemoryview checks that array.array and memoryview accept a
// weak reference the way CPython's C types do (they declare a __weakref__ slot),
// while the built-in containers with no slot still reject one.
func TestWeakrefArrayAndMemoryview(t *testing.T) {
	arr, err := NewArray(NewStr("i"), NewList([]Object{NewInt(1), NewInt(2)}))
	if err != nil {
		t.Fatalf("NewArray: %v", err)
	}
	mv, err := NewMemoryView(NewBytes([]byte("ab")))
	if err != nil {
		t.Fatalf("NewMemoryView: %v", err)
	}
	for _, tt := range []struct {
		name string
		obj  Object
	}{{"array", arr}, {"memoryview", mv}} {
		r, err := NewWeakref(tt.obj, nil)
		if err != nil {
			t.Fatalf("NewWeakref(%s): %v", tt.name, err)
		}
		if got := weakrefTarget(r.(*weakrefObject)); got != tt.obj {
			t.Errorf("%s: deref did not return the referent", tt.name)
		}
	}
	// A list and a dict carry no __weakref__ slot, so they still reject a ref.
	if _, err := NewWeakref(NewList(nil), nil); err == nil {
		t.Fatal("NewWeakref(list) did not raise, want TypeError")
	}
	d, _ := NewDict(nil, nil)
	if _, err := NewWeakref(d, nil); err == nil {
		t.Fatal("NewWeakref(dict) did not raise, want TypeError")
	}
}
