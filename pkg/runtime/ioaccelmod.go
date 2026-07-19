package runtime

import (
	"bytes"

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

	iobase, err := buildIOBase()
	if err != nil {
		panic("unagi: building _io._IOBase: " + err.Error())
	}
	ioIOBase = iobase

	moduleTable["_io"] = &moduleEntry{builtin: true, exec: initIOAccel}
}

func initIOAccel(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	if err := set("UnsupportedOperation", ioUnsupportedOperation); err != nil {
		return err
	}
	if err := set("_IOBase", ioIOBase); err != nil {
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

// ioIOBase is the singleton `_io._IOBase`, the abstract base every stream class
// derives from. It is a Go-constructed classObject carrying the default method
// surface as Python-visible methods, so vendored io.py's
// `class IOBase(_io._IOBase, metaclass=abc.ABCMeta)` inherits them through the
// ordinary MRO. The concrete streams are later sub-slices.
var ioIOBase objects.Object

// ioClosedAttr is the private instance attribute that marks a stream closed.
// CPython's _io._IOBase does the same: `closed` is true exactly when the
// instance carries this attribute, and close() sets it, so it shows up in the
// instance __dict__ after a close.
const ioClosedAttr = "__IOBase_closed"

// ioClosedMessage is the ValueError text every operation on a closed stream
// raises, matching CPython byte for byte.
const ioClosedMessage = "I/O operation on closed file."

// buildIOBase constructs the _io._IOBase classObject with its full default
// method surface. Each method is a self-binding NewMethod so a read off an
// instance passes the instance as self, the way a def-statement method does.
func buildIOBase() (objects.Object, error) {
	names := []string{
		"closed",
		"_checkClosed", "_checkReadable", "_checkWritable", "_checkSeekable",
		"readable", "writable", "seekable", "isatty",
		"flush", "close", "fileno",
		"seek", "tell", "truncate",
		"__enter__", "__exit__", "__iter__", "__next__", "__del__",
		"readline", "readlines", "writelines",
	}
	vals := []objects.Object{
		// closed is a read-only property: true once the instance carries the
		// private closed attribute, false before.
		objects.NewProperty(objects.NewFunc("closed", 1, func(args []objects.Object) (objects.Object, error) {
			return objects.NewBool(ioIsClosed(args[0])), nil
		}), nil, nil),
		// _checkClosed raises on a closed stream, else returns None; the guard the
		// other methods and the context/iterator entries lean on.
		ioMethod("_checkClosed", 1, func(args []objects.Object) (objects.Object, error) {
			if ioIsClosed(args[0]) {
				return nil, ioClosedError()
			}
			return objects.None, nil
		}),
		// _checkReadable/_checkWritable/_checkSeekable raise UnsupportedOperation
		// with the "File or stream is not %sable." message when the matching
		// predicate is false; they call the predicate through self so a subclass
		// override is honored.
		ioCheckMethod("_checkReadable", "readable", "File or stream is not readable."),
		ioCheckMethod("_checkWritable", "writable", "File or stream is not writable."),
		ioCheckMethod("_checkSeekable", "seekable", "File or stream is not seekable."),
		// readable/writable/seekable/isatty all report false on the bare base.
		ioConstMethod("readable", objects.False),
		ioConstMethod("writable", objects.False),
		ioConstMethod("seekable", objects.False),
		ioConstMethod("isatty", objects.False),
		// flush is a no-op on an open stream and raises on a closed one.
		ioMethod("flush", 1, func(args []objects.Object) (objects.Object, error) {
			if ioIsClosed(args[0]) {
				return nil, ioClosedError()
			}
			return objects.None, nil
		}),
		// close flushes then marks the stream closed; it is idempotent and keeps
		// the closed mark even if the flush raised, matching CPython.
		ioMethod("close", 1, func(args []objects.Object) (objects.Object, error) {
			self := args[0]
			if ioIsClosed(self) {
				return objects.None, nil
			}
			_, flushErr := objects.CallMethod(self, "flush", nil)
			if err := objects.StoreAttr(self, ioClosedAttr, objects.True); err != nil {
				return nil, err
			}
			if flushErr != nil {
				return nil, flushErr
			}
			return objects.None, nil
		}),
		// fileno/seek/truncate are unsupported on the bare base.
		ioUnsupportedMethod("fileno", "fileno"),
		ioUnsupportedMethod("seek", "seek"),
		// tell is seek(0, 1), so it surfaces whatever the stream's seek does; on
		// the bare base that is UnsupportedOperation: seek.
		ioMethod("tell", 1, func(args []objects.Object) (objects.Object, error) {
			return objects.CallMethod(args[0], "seek", []objects.Object{objects.NewInt(0), objects.NewInt(1)})
		}),
		ioUnsupportedMethod("truncate", "truncate"),
		// __enter__ checks the stream is open and returns it; __exit__ closes it
		// and does not suppress an exception.
		ioMethod("__enter__", 1, func(args []objects.Object) (objects.Object, error) {
			self := args[0]
			if _, err := objects.CallMethod(self, "_checkClosed", nil); err != nil {
				return nil, err
			}
			return self, nil
		}),
		ioMethod("__exit__", -1, func(args []objects.Object) (objects.Object, error) {
			if _, err := objects.CallMethod(args[0], "close", nil); err != nil {
				return nil, err
			}
			return objects.None, nil
		}),
		// __iter__ returns the stream itself after an open check; __next__ reads a
		// line and raises StopIteration on end of stream.
		ioMethod("__iter__", 1, func(args []objects.Object) (objects.Object, error) {
			self := args[0]
			if _, err := objects.CallMethod(self, "_checkClosed", nil); err != nil {
				return nil, err
			}
			return self, nil
		}),
		ioMethod("__next__", 1, func(args []objects.Object) (objects.Object, error) {
			line, err := objects.CallMethod(args[0], "readline", nil)
			if err != nil {
				return nil, err
			}
			n, err := objects.Len(line)
			if err != nil {
				return nil, err
			}
			if n == 0 {
				return nil, objects.NewException("StopIteration", nil)
			}
			return line, nil
		}),
		// __del__ closes the stream. GC and finalizers are not modeled, so this is
		// never fired automatically; a real close happens through an explicit
		// close() or the context manager.
		ioMethod("__del__", 1, func(args []objects.Object) (objects.Object, error) {
			if _, err := objects.CallMethod(args[0], "close", nil); err != nil {
				return nil, err
			}
			return objects.None, nil
		}),
		ioMethod("readline", -1, ioReadline),
		ioMethod("readlines", -1, ioReadlines),
		ioMethod("writelines", 2, ioWritelines),
	}
	return objects.NewClass("_IOBase", "_io._IOBase", nil, names, vals, nil, nil)
}

// ioMethod builds a self-binding _IOBase method. args[0] is the instance.
func ioMethod(name string, arity int, fn func(args []objects.Object) (objects.Object, error)) objects.Object {
	return objects.NewMethod(name, arity, fn)
}

// ioConstMethod builds a zero-argument method that always returns v, the shape
// of readable/writable/seekable/isatty on the bare base.
func ioConstMethod(name string, v objects.Object) objects.Object {
	return objects.NewMethod(name, 1, func([]objects.Object) (objects.Object, error) {
		return v, nil
	})
}

// ioUnsupportedMethod builds a method that always raises UnsupportedOperation
// with the given operation name, the default for an operation the bare base
// cannot perform.
func ioUnsupportedMethod(name, op string) objects.Object {
	return objects.NewMethod(name, -1, func([]objects.Object) (objects.Object, error) {
		return nil, ioUnsupported(op)
	})
}

// ioCheckMethod builds a _check* method that raises UnsupportedOperation with
// msg when the named predicate returns false on self.
func ioCheckMethod(name, predicate, msg string) objects.Object {
	return objects.NewMethod(name, -1, func(args []objects.Object) (objects.Object, error) {
		ok, err := objects.CallMethod(args[0], predicate, nil)
		if err != nil {
			return nil, err
		}
		if !objects.Truth(ok) {
			return nil, ioUnsupported(msg)
		}
		return objects.None, nil
	})
}

// ioIsClosed reports whether a stream instance carries the private closed
// attribute, the way CPython's closed property probes it.
func ioIsClosed(self objects.Object) bool {
	_, err := objects.LoadAttr(self, ioClosedAttr)
	return err == nil
}

// ioClosedError is the ValueError every operation on a closed stream raises.
func ioClosedError() error {
	return objects.Raise(objects.ValueError, "%s", ioClosedMessage)
}

// ioUnsupported builds an UnsupportedOperation carrying msg, raised for an
// operation a stream does not support.
func ioUnsupported(msg string) error {
	e, err := objects.Call(ioUnsupportedOperation, []objects.Object{objects.NewStr(msg)})
	if err != nil {
		return err
	}
	if exc, ok := e.(error); ok {
		return exc
	}
	return objects.Raise(objects.RuntimeError, "_io: UnsupportedOperation is not raisable")
}

// ioReadline reads one line as bytes, up to and including a trailing newline, or
// the whole remainder at end of stream, or the first size bytes when size is
// non-negative. It reads a byte at a time through self.read, or uses self.peek
// to read ahead when the stream provides it, matching CPython's base readline.
func ioReadline(args []objects.Object) (objects.Object, error) {
	self := args[0]
	size := int64(-1)
	if len(args) >= 2 && args[1] != objects.None {
		n, ok := objects.AsInt(args[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "size must be an integer")
		}
		size = n
	}
	hasPeek := ioHasAttr(self, "peek")
	var res []byte
	for size < 0 || int64(len(res)) < size {
		n := int64(1)
		if hasPeek {
			readahead, err := objects.CallMethod(self, "peek", []objects.Object{objects.NewInt(1)})
			if err != nil {
				return nil, err
			}
			ra, _ := objects.AsBytesLike(readahead)
			if len(ra) != 0 {
				if idx := bytes.IndexByte(ra, '\n'); idx >= 0 {
					n = int64(idx + 1)
				} else {
					n = int64(len(ra))
				}
				if size >= 0 && n > size {
					n = size
				}
			}
		}
		chunk, err := objects.CallMethod(self, "read", []objects.Object{objects.NewInt(n)})
		if err != nil {
			return nil, err
		}
		if !objects.Truth(chunk) {
			break
		}
		cb, ok := objects.AsBytesLike(chunk)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "read() should return bytes, not %s", chunk.TypeName())
		}
		res = append(res, cb...)
		if res[len(res)-1] == '\n' {
			break
		}
	}
	return objects.NewBytes(res), nil
}

// ioReadlines reads and returns a list of lines. A positive hint stops once the
// running total of bytes read reaches it; a missing or non-positive hint reads
// to end of stream.
func ioReadlines(args []objects.Object) (objects.Object, error) {
	self := args[0]
	hint := int64(-1)
	if len(args) >= 2 && args[1] != objects.None {
		if n, ok := objects.AsInt(args[1]); ok {
			hint = n
		}
	}
	var lines []objects.Object
	var total int64
	for {
		line, err := objects.CallMethod(self, "readline", nil)
		if err != nil {
			return nil, err
		}
		n, err := objects.Len(line)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			break
		}
		lines = append(lines, line)
		total += int64(n)
		if hint > 0 && total >= hint {
			break
		}
	}
	return objects.NewList(lines), nil
}

// ioWritelines writes each line of an iterable through self.write, after an open
// check. It returns None.
func ioWritelines(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if _, err := objects.CallMethod(self, "_checkClosed", nil); err != nil {
		return nil, err
	}
	it, err := objects.Iter(args[1])
	if err != nil {
		return nil, err
	}
	for {
		line, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		if _, err := objects.CallMethod(self, "write", []objects.Object{line}); err != nil {
			return nil, err
		}
	}
	return objects.None, nil
}

// ioHasAttr reports whether an object exposes an attribute, the hasattr the base
// readline uses to detect a peek-capable stream.
func ioHasAttr(o objects.Object, name string) bool {
	_, err := objects.LoadAttr(o, name)
	return err == nil
}
