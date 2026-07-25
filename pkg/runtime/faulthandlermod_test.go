package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestFaulthandlerModule checks the observable contract: the module imports,
// is_enabled starts False, enable() flips it to True and disable() back to
// False, and the diagnostic entry points accept their arguments without raising.
func TestFaulthandlerModule(t *testing.T) {
	// Reset the module-global so the test does not depend on prior state.
	faulthandlerState.mu.Lock()
	faulthandlerState.enabled = false
	faulthandlerState.mu.Unlock()

	mo, err := ImportModule("faulthandler")
	if err != nil {
		t.Fatalf("import faulthandler: %v", err)
	}
	call := func(name string, a ...objects.Object) objects.Object {
		t.Helper()
		fn, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("faulthandler.%s: %v", name, err)
		}
		v, err := objects.Call(fn, a)
		if err != nil {
			t.Fatalf("faulthandler.%s(): %v", name, err)
		}
		return v
	}

	if call("is_enabled") != objects.False {
		t.Fatalf("is_enabled() = True before enable, want False")
	}
	call("enable")
	if call("is_enabled") != objects.True {
		t.Fatalf("is_enabled() = False after enable, want True")
	}
	call("disable")
	if call("is_enabled") != objects.False {
		t.Fatalf("is_enabled() = True after disable, want False")
	}

	// The diagnostic entry points accept their arguments and do not raise.
	call("dump_traceback")
	call("dump_traceback_later", objects.NewFloat(5))
	call("cancel_dump_traceback_later")
	call("register", objects.NewInt(2))
	if call("unregister", objects.NewInt(2)) != objects.False {
		t.Fatalf("unregister() = True, want False")
	}
	call("dump_c_stack")
}
