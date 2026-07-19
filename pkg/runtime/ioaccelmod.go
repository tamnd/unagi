package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _io is the C accelerator behind the pure-Python io module. Vendored Lib/io.py
// opens with `import _io` and `from _io import (...)`, so the accelerator has to
// exist before io.py imports at all; io.py is not brought up on it yet, this is
// the first slice of the _io surface (Spec 2076 stdlib S0_io_arc.md).
//
// This slice stands up the module skeleton and its exception and constant
// surface: UnsupportedOperation, the class io.py re-exports for the operations a
// stream does not support, plus DEFAULT_BUFFER_SIZE and the BlockingIOError
// re-export. The _IOBase family and the concrete streams are later sub-slices.

// ioUnsupportedOperation is the singleton UnsupportedOperation class, built once
// so `from _io import UnsupportedOperation` and every read of the name resolve
// to the same object, the identity io.py preserves with its own re-export. It
// derives from both OSError and ValueError, so an except of either catches it,
// and reports itself as io.UnsupportedOperation the way CPython does.
var ioUnsupportedOperation objects.Object

func init() {
	base := func(name string) objects.Object {
		c, ok := objects.ExcClassValue(name)
		if !ok {
			panic("unagi: _io needs builtin exception " + name)
		}
		return c
	}
	cls, err := objects.NewClass(
		"UnsupportedOperation", "io.UnsupportedOperation",
		[]objects.Object{base("OSError"), base("ValueError")},
		[]string{"__module__"}, []objects.Object{objects.NewStr("io")},
		nil, nil,
	)
	if err != nil {
		panic("unagi: building _io.UnsupportedOperation: " + err.Error())
	}
	ioUnsupportedOperation = cls

	moduleTable["_io"] = &moduleEntry{builtin: true, exec: initIOAccel}
}

func initIOAccel(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	if err := set("UnsupportedOperation", ioUnsupportedOperation); err != nil {
		return err
	}
	// DEFAULT_BUFFER_SIZE is the buffer size the buffered streams and open()
	// default to; io.py re-exports it under the same name.
	if err := set("DEFAULT_BUFFER_SIZE", objects.NewInt(131072)); err != nil {
		return err
	}
	// BlockingIOError is a builtin exception _io only re-exports, so the name
	// resolves to the very object the builtin namespace binds.
	blocking, ok := objects.ExcClassValue("BlockingIOError")
	if !ok {
		return objects.Raise(objects.RuntimeError, "_io: BlockingIOError missing")
	}
	return set("BlockingIOError", blocking)
}
