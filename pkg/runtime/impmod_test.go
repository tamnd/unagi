package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestImpOverrideFrozenModulesForTests pins the shim test.support.import_helper's
// frozen_modules() context manager depends on. unagi freezes nothing, so the
// override does nothing, but the function must exist and return None or the
// context manager raises AttributeError on entry -- the gate a broad swath of
// Lib/test modules pass through when they call import_fresh_module.
func TestImpOverrideFrozenModulesForTests(t *testing.T) {
	mod, err := ImportModule("_imp")
	if err != nil {
		t.Fatalf("import _imp: %v", err)
	}
	fn, err := objects.LoadAttr(mod, "_override_frozen_modules_for_tests")
	if err != nil {
		t.Fatalf("_imp._override_frozen_modules_for_tests missing: %v", err)
	}
	// The three documented arguments -- on, off, and no-override -- each return
	// None the way CPython's internal-only primitive does.
	for _, arg := range []int64{1, -1, 0} {
		got, err := objects.Call(fn, []objects.Object{objects.NewInt(arg)})
		if err != nil {
			t.Fatalf("_override_frozen_modules_for_tests(%d): %v", arg, err)
		}
		if got != objects.None {
			t.Errorf("_override_frozen_modules_for_tests(%d) = %s, want None", arg, objects.Repr(got))
		}
	}
}
