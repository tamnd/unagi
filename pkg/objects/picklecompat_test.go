package objects

import "testing"

// TestCompatReverseGlobalPrecedence pins the save-side precedence CPython uses
// under fix_imports: a whole-name entry wins and rewrites both fields, a module
// with no name entry is mapped alone, and an unmapped module passes through.
func TestCompatReverseGlobalPrecedence(t *testing.T) {
	cases := []struct{ module, name, wantMod, wantName string }{
		{"builtins", "int", "__builtin__", "long"},             // name entry rewrites both
		{"builtins", "str", "__builtin__", "unicode"},          // name entry rewrites both
		{"builtins", "map", "itertools", "imap"},               // name entry crosses module
		{"builtins", "ValueError", "exceptions", "ValueError"}, // exception moves module
		{"builtins", "len", "__builtin__", "len"},              // module-only fallback
		{"builtins", "object", "__builtin__", "object"},        // module-only fallback
		{"__main__", "Foo", "__main__", "Foo"},                 // no mapping, unchanged
		{"mymod", "thing", "mymod", "thing"},                   // no mapping, unchanged
	}
	for _, c := range cases {
		m, n := compatReverseGlobal(c.module, c.name)
		if m != c.wantMod || n != c.wantName {
			t.Errorf("compatReverseGlobal(%q,%q) = (%q,%q), want (%q,%q)",
				c.module, c.name, m, n, c.wantMod, c.wantName)
		}
	}
}

// TestCompatForwardGlobalPrecedence pins the load-side inverse: an old whole-name
// global maps back to its modern spelling, a bare Python-2 module maps forward,
// and a modern name passes through untouched.
func TestCompatForwardGlobalPrecedence(t *testing.T) {
	cases := []struct{ module, name, wantMod, wantName string }{
		{"__builtin__", "long", "builtins", "int"},
		{"__builtin__", "unicode", "builtins", "str"},
		{"itertools", "imap", "builtins", "map"},
		{"exceptions", "ValueError", "builtins", "ValueError"},
		{"__builtin__", "len", "builtins", "len"}, // module-only fallback
		{"builtins", "int", "builtins", "int"},    // already modern, unchanged
		{"__main__", "Foo", "__main__", "Foo"},    // no mapping, unchanged
	}
	for _, c := range cases {
		m, n := compatForwardGlobal(c.module, c.name)
		if m != c.wantMod || n != c.wantName {
			t.Errorf("compatForwardGlobal(%q,%q) = (%q,%q), want (%q,%q)",
				c.module, c.name, m, n, c.wantMod, c.wantName)
		}
	}
}

// TestCompatNameTablesRoundTrip confirms every reverse name entry maps back to a
// builtins-namespace global through the forward table, so a protocol-2 pickle of a
// builtin resolves to the same object it named. The reverse table is lossy where
// several Python-3 exceptions collapse onto one Python-2 name (OSError and its
// subclasses), so the forward image need only land back in the builtins module,
// not on the exact original name.
func TestCompatNameTablesRoundTrip(t *testing.T) {
	for k, v := range compatReverseName {
		if k.module != "builtins" {
			continue
		}
		fm, fn := compatForwardGlobal(v[0], v[1])
		if fm != "builtins" {
			t.Errorf("reverse %v -> %v forwards to module %q, want builtins", k, v, fm)
		}
		// The non-collapsing entries must forward back to the exact original name.
		if v[0] != "exceptions" && fn != k.name {
			t.Errorf("reverse %v -> %v forwards to name %q, want %q", k, v, fn, k.name)
		}
	}
}
