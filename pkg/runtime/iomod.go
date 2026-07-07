package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// io is a built-in module: StringIO and BytesIO are C types in CPython, so the
// runtime provides them in Go behind the io import. The constructors here build
// the in-memory streams; the read, write, seek and context-manager surface
// lives on the objects themselves.

func init() {
	moduleTable["io"] = &moduleEntry{builtin: true, exec: initIO}
}

func initIO(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	if err := set("StringIO", objects.NewFunc("StringIO", -1, ioStringIO)); err != nil {
		return err
	}
	return set("BytesIO", objects.NewFunc("BytesIO", -1, ioBytesIO))
}

// ioStringIO builds an io.StringIO. The optional initial value must be a str or
// None, the type CPython insists on.
func ioStringIO(args []objects.Object) (objects.Object, error) {
	if len(args) > 1 {
		return nil, objects.Raise(objects.TypeError, "StringIO() takes at most 1 argument (%d given)", len(args))
	}
	initial := ""
	if len(args) == 1 && args[0] != objects.None {
		s, ok := objects.AsStr(args[0])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "initial_value must be str or None, not %s", args[0].TypeName())
		}
		initial = s
	}
	return objects.NewStringIO(initial), nil
}

// ioBytesIO builds an io.BytesIO. The optional initial value must be a
// bytes-like object.
func ioBytesIO(args []objects.Object) (objects.Object, error) {
	if len(args) > 1 {
		return nil, objects.Raise(objects.TypeError, "BytesIO() takes at most 1 argument (%d given)", len(args))
	}
	var initial []byte
	if len(args) == 1 && args[0] != objects.None {
		b, ok := objects.AsBytesLike(args[0])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "a bytes-like object is required, not '%s'", args[0].TypeName())
		}
		initial = b
	}
	return objects.NewBytesIO(initial), nil
}
