package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestBuiltinsModuleExposesNamespace checks the builtins module carries the
// builtin namespace: a function from the table, a descriptor constructor the
// table does not hold, an exception class, and the keyword constants a program
// reaches only through getattr.
func TestBuiltinsModuleExposesNamespace(t *testing.T) {
	mo, err := ImportModule("builtins")
	if err != nil {
		t.Fatalf("import builtins: %v", err)
	}
	get := func(name string) objects.Object {
		v, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("builtins.%s: %v", name, err)
		}
		return v
	}

	// A table function is the same object the bare name resolves to.
	if fn, ok := Builtin("len"); !ok || get("len") != fn {
		t.Error("builtins.len is not the runtime len")
	}
	// property comes from the descriptor singletons, not the table.
	if get("property") != objects.PropertyBuiltin {
		t.Error("builtins.property is not PropertyBuiltin")
	}
	// An exception class is the same object the name binds.
	if cls, ok := objects.ExcClassValue("ValueError"); !ok || get("ValueError") != cls {
		t.Error("builtins.ValueError is not the ValueError class")
	}
	// The keyword constants ride along for getattr and from-import.
	if get("None") != objects.None || get("True") != objects.True || get("False") != objects.False {
		t.Error("builtins is missing a keyword constant")
	}
	if get("__debug__") != objects.True {
		t.Error("builtins.__debug__ should be True")
	}
	if get("Ellipsis") != objects.Ellipsis || get("NotImplemented") != objects.NotImplemented {
		t.Error("builtins is missing Ellipsis or NotImplemented")
	}
}
