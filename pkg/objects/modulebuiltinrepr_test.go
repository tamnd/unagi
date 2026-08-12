package objects

import "testing"

// TestBuiltinModuleRepr checks a runtime-provided module reprs from its origin
// as "<module 'name' (built-in)>", the way CPython renders a module with no
// source file, while an ordinary source module still reprs from its file.
func TestBuiltinModuleRepr(t *testing.T) {
	b := NewBuiltinModule("sys")
	if got, want := Repr(b), "<module 'sys' (built-in)>"; got != want {
		t.Fatalf("builtin module repr = %q; want %q", got, want)
	}

	m := NewModule("pkg", "/tmp/pkg.py")
	if got, want := Repr(m), "<module 'pkg' from '/tmp/pkg.py'>"; got != want {
		t.Fatalf("source module repr = %q; want %q", got, want)
	}
}
