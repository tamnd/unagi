package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestImportMainModuleStandIn checks that `import __main__` resolves to an empty
// stand-in when the program never registered its module, and that the stand-in
// has a valid __dict__ and identity name.
func TestImportMainModuleStandIn(t *testing.T) {
	mainModule = nil
	modulesDel("__main__")

	m, err := ImportModule("__main__")
	if err != nil {
		t.Fatalf("import __main__: %v", err)
	}
	if m.TypeName() != "module" {
		t.Fatalf("type = %s, want module", m.TypeName())
	}
	name, err := objects.LoadAttr(m, "__name__")
	if err != nil {
		t.Fatalf("__name__: %v", err)
	}
	if s, _ := objects.AsStr(name); s != "__main__" {
		t.Errorf("__name__ = %q, want __main__", s)
	}
	d, err := objects.LoadAttr(m, "__dict__")
	if err != nil {
		t.Fatalf("__dict__: %v", err)
	}
	if d.TypeName() != "dict" {
		t.Errorf("__dict__ type = %s, want dict", d.TypeName())
	}
	// A second import returns the same object, the stable sys.modules entry.
	m2, err := ImportModule("__main__")
	if err != nil {
		t.Fatalf("re-import __main__: %v", err)
	}
	if m2 != m {
		t.Errorf("__main__ import is not stable across calls")
	}
}

// TestSetMainModule checks that SetMainModule makes `import __main__` hand back
// the registered module, the live top-level namespace the generated main builds.
func TestSetMainModule(t *testing.T) {
	mainModule = nil
	modulesDel("__main__")

	real := objects.NewModule("__main__", "prog.py")
	real.SetGlobal("SENTINEL", objects.NewInt(7))
	SetMainModule(real)

	m, err := ImportModule("__main__")
	if err != nil {
		t.Fatalf("import __main__: %v", err)
	}
	if m != objects.Object(real) {
		t.Fatalf("import __main__ did not return the registered module")
	}
	v, err := objects.LoadAttr(m, "SENTINEL")
	if err != nil {
		t.Fatalf("SENTINEL: %v", err)
	}
	if n, _ := objects.AsInt(v); n != 7 {
		t.Errorf("SENTINEL = %d, want 7", n)
	}

	mainModule = nil
	modulesDel("__main__")
}
