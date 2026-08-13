package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// mustDefaultDict builds a defaultdict with the given factory and pairs. The
// collections init registers collections.defaultdict for pickling and the int
// factory pickles as a builtins global, so a reduction names both and resolves
// them back without an import.
func mustDefaultDict(t *testing.T, factory objects.Object, keys, vals []objects.Object) objects.Object {
	t.Helper()
	dd, err := objects.NewDefaultDict(factory, keys, vals)
	if err != nil {
		t.Fatalf("NewDefaultDict: %v", err)
	}
	return dd
}

// TestDefaultDictPickleRoundTrip confirms a defaultdict survives dumps/loads at
// every binary protocol, coming back a defaultdict with the same factory, the
// same pairs in insertion order, and the factory still live so a missing key
// materializes. It reduces through the five-tuple defaultdict.__reduce_ex__
// emits, (defaultdict_type, (factory,), None, None, items_iterator), whose item
// iterator the pickler replays as setitems onto the reconstructed mapping.
func TestDefaultDictPickleRoundTrip(t *testing.T) {
	intFactory := mustBuiltin(t, "int")
	keys := []objects.Object{objects.NewStr("a"), objects.NewStr("b"), objects.NewStr("c")}
	vals := []objects.Object{objects.NewInt(1), objects.NewInt(2), objects.NewInt(3)}
	dd := mustDefaultDict(t, intFactory, keys, vals)
	for _, proto := range []int{2, 3, 4, 5} {
		data, err := objects.PickleDumps(dd, proto)
		if err != nil {
			t.Fatalf("dumps(proto=%d): %v", proto, err)
		}
		back, err := objects.PickleLoads(data)
		if err != nil {
			t.Fatalf("loads(proto=%d): %v", proto, err)
		}
		if back.TypeName() != dd.TypeName() {
			t.Fatalf("loads(proto=%d) = %s, want %s", proto, back.TypeName(), dd.TypeName())
		}
		// The factory comes back the same builtin singleton.
		factory, err := objects.LoadAttr(back, "default_factory")
		if err != nil {
			t.Fatalf("default_factory(proto=%d): %v", proto, err)
		}
		if factory != intFactory {
			t.Fatalf("default_factory(proto=%d) = %v, want int", proto, factory)
		}
		// A missing key materializes through the restored factory, giving 0.
		got, err := objects.GetItem(back, objects.NewStr("missing"))
		if err != nil {
			t.Fatalf("getitem(proto=%d): %v", proto, err)
		}
		if n, ok := objects.AsIntValue(got); !ok || n != 0 {
			t.Fatalf("missing key(proto=%d) = %v, want 0", proto, got)
		}
	}
}

// TestDefaultDictPickleNoFactory confirms a defaultdict with no factory reduces
// through the empty argument tuple, so it comes back with default_factory None
// and a missing key still raises KeyError the way an unset factory does.
func TestDefaultDictPickleNoFactory(t *testing.T) {
	dd := mustDefaultDict(t, objects.None,
		[]objects.Object{objects.NewStr("x")}, []objects.Object{objects.NewInt(9)})
	data, err := objects.PickleDumps(dd, 5)
	if err != nil {
		t.Fatalf("dumps: %v", err)
	}
	back, err := objects.PickleLoads(data)
	if err != nil {
		t.Fatalf("loads: %v", err)
	}
	factory, err := objects.LoadAttr(back, "default_factory")
	if err != nil {
		t.Fatalf("default_factory: %v", err)
	}
	if factory != objects.None {
		t.Fatalf("default_factory = %v, want None", factory)
	}
	if _, err := objects.GetItem(back, objects.NewStr("nope")); err == nil {
		t.Fatalf("missing key on a factory-less defaultdict succeeded, want KeyError")
	}
}

// TestDefaultDictReduceArity pins the object-inherited arity errors: __reduce_ex__
// takes exactly its protocol argument and __reduce__ takes none.
func TestDefaultDictReduceArity(t *testing.T) {
	dd := mustDefaultDict(t, mustBuiltin(t, "int"), nil, nil)
	if _, err := objects.CallMethod(dd, "__reduce_ex__", nil); err == nil {
		t.Fatalf("__reduce_ex__() with no protocol succeeded, want a TypeError")
	}
	if _, err := objects.CallMethod(dd, "__reduce__", []objects.Object{objects.NewInt(2)}); err == nil {
		t.Fatalf("__reduce__(2) succeeded, want a TypeError")
	}
}
