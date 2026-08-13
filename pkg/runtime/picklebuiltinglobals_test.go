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

// TestBuiltinGlobalProto2CompatNames pins the byte shape of a protocol-2 builtin
// global: fix_imports is on below protocol 3, so the reference goes out under its
// Python-2 spelling. int writes as __builtin__.long, map as itertools.imap and a
// builtin exception under the exceptions module, matching CPython byte-for-byte;
// protocol 3 keeps the modern builtins name. Each still loads back to the live
// singleton the reference named, both directions of the compat tables exercised.
func TestBuiltinGlobalProto2CompatNames(t *testing.T) {
	cases := []struct {
		name  string
		proto int
		want  []byte
	}{
		{"int", 2, []byte("\x80\x02c__builtin__\nlong\nq\x00.")},
		{"str", 2, []byte("\x80\x02c__builtin__\nunicode\nq\x00.")},
		{"map", 2, []byte("\x80\x02citertools\nimap\nq\x00.")},
		{"len", 2, []byte("\x80\x02c__builtin__\nlen\nq\x00.")},
		{"ValueError", 2, []byte("\x80\x02cexceptions\nValueError\nq\x00.")},
		{"int", 3, []byte("\x80\x03cbuiltins\nint\nq\x00.")},
		{"map", 3, []byte("\x80\x03cbuiltins\nmap\nq\x00.")},
	}
	for _, c := range cases {
		b := mustBuiltin(t, c.name)
		data, err := objects.PickleDumps(b, c.proto)
		if err != nil {
			t.Fatalf("dumps(%s, proto=%d): %v", c.name, c.proto, err)
		}
		if !bytes.Equal(data, c.want) {
			t.Fatalf("dumps(%s, proto=%d) = %q, want %q", c.name, c.proto, data, c.want)
		}
		back, err := objects.PickleLoads(data)
		if err != nil {
			t.Fatalf("loads(%s, proto=%d): %v", c.name, c.proto, err)
		}
		if back != b {
			t.Fatalf("round-trip(%s, proto=%d) = %v, want the same singleton", c.name, c.proto, back)
		}
	}
}

// TestBuiltinGlobalLoadsCPythonProto2 confirms the loader resolves a Python-2
// global that CPython would write, mapping the old name forward to the live
// builtin. These are the exact bytes CPython emits at protocol 2, fed straight in.
func TestBuiltinGlobalLoadsCPythonProto2(t *testing.T) {
	cases := []struct {
		data []byte
		name string
	}{
		{[]byte("\x80\x02c__builtin__\nlong\nq\x00."), "int"},
		{[]byte("\x80\x02c__builtin__\nunicode\nq\x00."), "str"},
		{[]byte("\x80\x02citertools\nimap\nq\x00."), "map"},
		{[]byte("\x80\x02citertools\nifilter\nq\x00."), "filter"},
		{[]byte("\x80\x02c__builtin__\nxrange\nq\x00."), "range"},
		{[]byte("\x80\x02cexceptions\nValueError\nq\x00."), "ValueError"},
	}
	for _, c := range cases {
		back, err := objects.PickleLoads(c.data)
		if err != nil {
			t.Fatalf("loads(%s): %v", c.name, err)
		}
		if back != mustBuiltin(t, c.name) {
			t.Fatalf("loads(%q) = %v, want %s", c.data, back, c.name)
		}
	}
}

// TestBuiltinGlobalAliasCanonical pins the canonical name for a builtin type that
// is reachable under several names: IOError and EnvironmentError both alias the
// OSError singleton, and the namer must pick OSError, the type's __qualname__, the
// way CPython does. Pickling any of the three aliases yields the same OSError
// global, deterministically across repeated runs.
func TestBuiltinGlobalAliasCanonical(t *testing.T) {
	want := []byte("\x80\x02cexceptions\nOSError\nq\x00.")
	for _, name := range []string{"OSError", "IOError", "EnvironmentError"} {
		b, ok := Builtin(name)
		if !ok {
			t.Fatalf("Builtin(%q): not found", name)
		}
		for i := 0; i < 8; i++ {
			data, err := objects.PickleDumps(b, 2)
			if err != nil {
				t.Fatalf("dumps(%s): %v", name, err)
			}
			if !bytes.Equal(data, want) {
				t.Fatalf("dumps(%s) = %q, want the canonical %q", name, data, want)
			}
		}
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
