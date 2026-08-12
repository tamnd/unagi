package runtime

import (
	"bytes"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// mustBuiltin fetches a builtin by name, the singleton a program reads when it
// names int, len or object. The runtime init wires the pickle reverse-name and
// forward-name resolvers to this table, so no import is needed to pickle one.
func mustBuiltin(t *testing.T, name string) objects.Object {
	t.Helper()
	b, ok := Builtin(name)
	if !ok {
		t.Fatalf("Builtin(%q): not found", name)
	}
	return b
}

// TestBuiltinGlobalRoundTrip confirms a builtin type or function pickles as a
// bare global reference and loads back to the very same singleton, across every
// binary protocol. CPython saves each as a builtins.<name> global off its
// __module__/__qualname__; the transpiled builtins carry no such metadata, so the
// runtime supplies the name both directions and the reference round-trips to the
// object the pickler named rather than a stand-in.
func TestBuiltinGlobalRoundTrip(t *testing.T) {
	names := []string{
		"int", "float", "str", "bytes", "bytearray", "bool", "complex",
		"list", "dict", "set", "frozenset", "tuple", "object", "type",
		"len", "abs", "sorted", "reversed", "map", "filter", "zip",
		"enumerate", "range", "slice", "memoryview", "super",
		"staticmethod", "classmethod", "property", "repr", "getattr",
	}
	for _, proto := range []int{2, 3, 4, 5} {
		for _, name := range names {
			b := mustBuiltin(t, name)
			data, err := objects.PickleDumps(b, proto)
			if err != nil {
				t.Fatalf("dumps(%s, proto=%d): %v", name, proto, err)
			}
			back, err := objects.PickleLoads(data)
			if err != nil {
				t.Fatalf("loads(%s, proto=%d): %v", name, proto, err)
			}
			if back != b {
				t.Fatalf("round-trip(%s, proto=%d) = %v, want the same singleton", name, proto, back)
			}
		}
	}
}

// TestBuiltinGlobalModuleMemo pins the byte shape of the module-name memo: a
// builtin type and a builtin function each carry their own 'builtins' string, so
// the type globals memo-share one and the function globals memo-share a separate
// one. Pickling a tuple of two functions and then a type shows the second
// function fetch its shared module name back while the type writes its own.
func TestBuiltinGlobalModuleMemo(t *testing.T) {
	tup := objects.NewTuple([]objects.Object{
		mustBuiltin(t, "len"), mustBuiltin(t, "abs"), mustBuiltin(t, "int"),
	})
	data, err := objects.PickleDumps(tup, 4)
	if err != nil {
		t.Fatalf("dumps: %v", err)
	}
	// len writes 'builtins' fresh, abs fetches it back (BINGET), int writes a
	// second 'builtins' fresh for the type group.
	if got := bytes.Count(data, []byte("builtins")); got != 2 {
		t.Fatalf("module-name occurrences = %d, want 2 (one per group)", got)
	}
	back, err := objects.PickleLoads(data)
	if err != nil {
		t.Fatalf("loads: %v", err)
	}
	bt, ok := back.(interface{ TypeName() string })
	if !ok || bt.TypeName() != "tuple" {
		t.Fatalf("loads = %T, want a tuple", back)
	}
}

// TestBuiltinGlobalRefusesUnregistered confirms an object with no reachable
// builtin name is still refused with CPython's TypeError rather than pickled as a
// bogus global. A funcObject the runtime never exposed in the builtins table has
// no (module, qualname), so it cannot be referenced by name.
func TestBuiltinGlobalRefusesUnregistered(t *testing.T) {
	orphan := objects.NewFunc("not_a_builtin", 0, func([]objects.Object) (objects.Object, error) {
		return objects.None, nil
	})
	if _, err := objects.PickleDumps(orphan, 5); err == nil {
		t.Fatalf("dumps(orphan) succeeded, want a TypeError")
	}
}
