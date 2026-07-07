package runtime

import "github.com/tamnd/unagi/pkg/objects"

// builtins is a built-in module the same way sys is: `import builtins` binds a
// module whose attributes are the builtin namespace, the names a program can
// reach without importing anything. Floor modules lean on it, enum among them,
// which imports builtins to reach bin and property through a stable name even
// when a subclass shadows them.
//
// The attributes come straight from the runtime's own builtin table, the one
// name resolution reads, so the module stays in step with the builtins the rest
// of the runtime registers: the functions, the type objects, and the exception
// classes all land here without a second list to keep in sync. The descriptor
// constructors the compiler resolves to their singleton objects rather than the
// table, property, staticmethod, and classmethod, are added alongside, and so
// are the literal constants it emits directly rather than looking up, None,
// True, False, NotImplemented, Ellipsis, and __debug__, so a lookup through the
// module or a `from builtins import ...` finds every one of them.
func init() {
	moduleTable["builtins"] = &moduleEntry{builtin: true, exec: initBuiltins}
}

func initBuiltins(m *objects.Module) error {
	for name, obj := range builtins {
		if err := objects.StoreAttr(m, name, obj); err != nil {
			return err
		}
	}
	descriptors := []struct {
		name string
		val  objects.Object
	}{
		{"property", objects.PropertyBuiltin},
		{"staticmethod", objects.StaticMethodBuiltin},
		{"classmethod", objects.ClassMethodBuiltin},
	}
	for _, d := range descriptors {
		if err := objects.StoreAttr(m, d.name, d.val); err != nil {
			return err
		}
	}
	consts := []struct {
		name string
		val  objects.Object
	}{
		{"None", objects.None},
		{"True", objects.True},
		{"False", objects.False},
		{"NotImplemented", objects.NotImplemented},
		{"Ellipsis", objects.Ellipsis},
		{"__debug__", objects.True},
	}
	for _, c := range consts {
		if err := objects.StoreAttr(m, c.name, c.val); err != nil {
			return err
		}
	}
	return nil
}
