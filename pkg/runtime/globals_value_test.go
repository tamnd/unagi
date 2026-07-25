package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestGlobalsValueForm checks that globals read as a value (as timeit does with
// `_globals = globals`) resolves to a real callable that hands back the
// top-level module's namespace, rather than panicking through BuiltinFn.
func TestGlobalsValueForm(t *testing.T) {
	mainModule = nil
	modulesDel("__main__")
	real := objects.NewModule("__main__", "prog.py")
	real.SetGlobal("SENTINEL", objects.NewInt(7))
	SetMainModule(real)
	defer func() { mainModule = nil; modulesDel("__main__") }()

	g, ok := Builtin("globals")
	if !ok {
		t.Fatal("globals is not registered as a value builtin")
	}
	d, err := objects.Call(g, nil)
	if err != nil {
		t.Fatalf("globals(): %v", err)
	}
	if d.TypeName() != "dict" {
		t.Fatalf("globals() type = %s, want dict", d.TypeName())
	}
	v, err := objects.GetItem(d, objects.NewStr("SENTINEL"))
	if err != nil {
		t.Fatalf("globals()[SENTINEL]: %v", err)
	}
	if n, _ := objects.AsInt(v); n != 7 {
		t.Errorf("globals()[SENTINEL] = %d, want 7", n)
	}
	// An argument is the same TypeError CPython reports.
	if _, err := objects.Call(g, []objects.Object{objects.None}); err == nil {
		t.Error("globals(x) did not raise")
	}
}

// TestDirValueForm checks that dir read as a value resolves to a callable: the
// one-argument form lists an object's names, and the no-argument form lists the
// top-level module's names instead of panicking.
func TestDirValueForm(t *testing.T) {
	mainModule = nil
	modulesDel("__main__")
	real := objects.NewModule("__main__", "prog.py")
	real.SetGlobal("SENTINEL", objects.NewInt(7))
	SetMainModule(real)
	defer func() { mainModule = nil; modulesDel("__main__") }()

	d, ok := Builtin("dir")
	if !ok {
		t.Fatal("dir is not registered as a value builtin")
	}
	// No-argument form lists the main module namespace.
	names, err := objects.Call(d, nil)
	if err != nil {
		t.Fatalf("dir(): %v", err)
	}
	if !listContains(t, names, "SENTINEL") {
		t.Error("dir() did not list the main module name SENTINEL")
	}
	// One-argument form lists the given object's own names, not the module's.
	other := objects.NewModule("other", "other.py")
	other.SetGlobal("OTHER_NAME", objects.None)
	got, err := objects.Call(d, []objects.Object{other})
	if err != nil {
		t.Fatalf("dir(other): %v", err)
	}
	if !listContains(t, got, "OTHER_NAME") {
		t.Error("dir(other) did not list the argument's name OTHER_NAME")
	}
	if listContains(t, got, "SENTINEL") {
		t.Error("dir(other) leaked the main module name SENTINEL")
	}
}

func listContains(t *testing.T, lst objects.Object, want string) bool {
	t.Helper()
	it, err := objects.Iter(lst)
	if err != nil {
		t.Fatalf("iter: %v", err)
	}
	for {
		v, ok, err := it.Next()
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if !ok {
			return false
		}
		if s, _ := objects.AsStr(v); s == want {
			return true
		}
	}
}
