package objects

import (
	"bytes"
	"testing"
)

// stubBuiltin builds a funcObject the way a runtime builtin module would and
// registers it under (module, qualname) so it pickles as a global reference.
func stubBuiltin(module, qualname string) Object {
	fn := NewFuncKw(qualname, func(pos []Object, _ []string, _ []Object) (Object, error) {
		return None, nil
	})
	RegisterPickleBuiltin(module, qualname, fn)
	return fn
}

// TestPickleBuiltinGlobalRoundTrip confirms a registered builtin function comes
// back as the very same object at every binary protocol, the reference form a
// builtin type or function pickles under.
func TestPickleBuiltinGlobalRoundTrip(t *testing.T) {
	fn := stubBuiltin("stubmod", "stub_builtin")
	for _, proto := range []int{2, 3, 4, 5} {
		data, err := PickleDumps(fn, proto)
		if err != nil {
			t.Fatalf("dumps(proto=%d): %v", proto, err)
		}
		back, err := PickleLoads(data)
		if err != nil {
			t.Fatalf("loads(proto=%d): %v", proto, err)
		}
		if back != fn {
			t.Fatalf("loads(proto=%d) = %s, want the registered builtin", proto, Repr(back))
		}
	}
}

// TestPickleBuiltinFreshModule confirms two builtins from one module each write
// their module name fresh rather than sharing it through the memo, matching how
// CPython pickles C-module globals: their __module__ strings are distinct objects,
// so the name 'stubmod2' is emitted twice and both references still round-trip.
func TestPickleBuiltinFreshModule(t *testing.T) {
	a := stubBuiltin("stubmod2", "alpha")
	b := stubBuiltin("stubmod2", "beta")
	data, err := PickleDumps(NewTuple([]Object{a, b}), 4)
	if err != nil {
		t.Fatalf("dumps: %v", err)
	}
	if n := bytes.Count(data, []byte("stubmod2")); n != 2 {
		t.Fatalf("module name written %d times, want 2 (fresh per builtin global)", n)
	}
	back, err := PickleLoads(data)
	if err != nil {
		t.Fatalf("loads: %v", err)
	}
	tup, ok := back.(*tupleObject)
	if !ok || len(tup.elts) != 2 || tup.elts[0] != a || tup.elts[1] != b {
		t.Fatalf("loads = %s, want (alpha, beta) as the registered builtins", Repr(back))
	}
}

// TestPickleBuiltinUnregistered confirms a builtin function the registry does not
// know still refuses to pickle with the same TypeError a non-picklable object
// gets, so the by-name path never invents a reference that cannot resolve.
func TestPickleBuiltinUnregistered(t *testing.T) {
	fn := NewFuncKw("loose_builtin", func(_ []Object, _ []string, _ []Object) (Object, error) {
		return None, nil
	})
	if _, err := PickleDumps(fn, 5); err == nil {
		t.Fatal("dumps of an unregistered builtin did not raise")
	}
}
