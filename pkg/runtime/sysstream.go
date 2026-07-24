package runtime

import (
	"io"

	"github.com/tamnd/unagi/pkg/objects"
)

// sys.stdout, sys.stderr and sys.stdin are the process standard streams. CPython
// exposes them as _io.TextIOWrapper objects; a compiled unagi program has no
// real io stack behind them, so this builds a minimal text stream whose write
// and flush route to the swappable runtime.Stdout and runtime.Stderr writers the
// print builtins already use. It carries just enough surface for the stdlib that
// reaches sys.stdout directly, such as pprint writing through _stream.write.

var sysStreamClass objects.Object

// buildSysStreams builds the three standard-stream objects and returns them in
// stdout, stderr, stdin order. They share one class; each instance carries the
// file descriptor that selects its writer and its readable/writable direction.
func buildSysStreams() (out, err, in objects.Object, buildErr error) {
	if sysStreamClass == nil {
		cls, e := newSysStreamClass()
		if e != nil {
			return nil, nil, nil, e
		}
		sysStreamClass = cls
	}
	out, buildErr = objects.Call(sysStreamClass,
		[]objects.Object{objects.NewInt(1), objects.NewStr("<stdout>"), objects.NewStr("w")})
	if buildErr != nil {
		return nil, nil, nil, buildErr
	}
	err, buildErr = objects.Call(sysStreamClass,
		[]objects.Object{objects.NewInt(2), objects.NewStr("<stderr>"), objects.NewStr("w")})
	if buildErr != nil {
		return nil, nil, nil, buildErr
	}
	in, buildErr = objects.Call(sysStreamClass,
		[]objects.Object{objects.NewInt(0), objects.NewStr("<stdin>"), objects.NewStr("r")})
	if buildErr != nil {
		return nil, nil, nil, buildErr
	}
	return out, err, in, nil
}

func newSysStreamClass() (objects.Object, error) {
	names := []string{
		"__init__", "__repr__", "write", "writelines", "flush", "close",
		"writable", "readable", "seekable", "isatty", "fileno",
	}
	vals := []objects.Object{
		objects.NewMethod("__init__", 4, sysStreamInit),
		objects.NewMethod("__repr__", 1, sysStreamRepr),
		objects.NewMethod("write", 2, sysStreamWrite),
		objects.NewMethod("writelines", 2, sysStreamWritelines),
		objects.NewMethod("flush", 1, sysStreamFlush),
		objects.NewMethod("close", 1, sysStreamFlush),
		objects.NewMethod("writable", 1, sysStreamWritable),
		objects.NewMethod("readable", 1, sysStreamReadable),
		objects.NewMethod("seekable", 1, sysStreamFalse),
		objects.NewMethod("isatty", 1, sysStreamFalse),
		objects.NewMethod("fileno", 1, sysStreamFileno),
	}
	return objects.NewClass("TextIOWrapper", "_io.TextIOWrapper",
		nil, names, vals, nil, nil)
}

// sysStreamInit stores the descriptor, the display name, and the fixed text
// attributes CPython reports for a standard stream.
func sysStreamInit(args []objects.Object) (objects.Object, error) {
	self, fd, name, mode := args[0], args[1], args[2], args[3]
	for _, kv := range []struct {
		name string
		v    objects.Object
	}{
		{"__fd__", fd},
		{"name", name},
		{"mode", mode},
		{"encoding", objects.NewStr("utf-8")},
		{"errors", objects.NewStr("strict")},
		{"closed", objects.False},
	} {
		if err := objects.StoreAttr(self, kv.name, kv.v); err != nil {
			return nil, err
		}
	}
	return objects.None, nil
}

// sysStreamWriter selects the process writer for a stream by its descriptor, so
// a host or test that swaps runtime.Stdout is honored at write time.
func sysStreamWriter(self objects.Object) io.Writer {
	fd, err := objects.LoadAttr(self, "__fd__")
	if err == nil {
		if n, ok := objects.AsInt(fd); ok && n == 2 {
			return Stderr
		}
	}
	return Stdout
}

func sysStreamWrite(args []objects.Object) (objects.Object, error) {
	self, s := args[0], args[1]
	str, ok := objects.AsStr(s)
	if !ok {
		return nil, objects.Raise(objects.TypeError,
			"write() argument must be str, not %s", s.TypeName())
	}
	if _, err := io.WriteString(sysStreamWriter(self), str); err != nil {
		return nil, objects.Raise("OSError", "%s", err.Error())
	}
	return objects.NewInt(int64(len([]rune(str)))), nil
}

func sysStreamWritelines(args []objects.Object) (objects.Object, error) {
	self, lines := args[0], args[1]
	it, err := objects.Iter(lines)
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
		if _, err := sysStreamWrite([]objects.Object{self, line}); err != nil {
			return nil, err
		}
	}
	return objects.None, nil
}

func sysStreamFlush(args []objects.Object) (objects.Object, error) {
	// The underlying writer is unbuffered here, and close on a standard stream is
	// a no-op the way CPython keeps the real fd open, so both settle to None.
	return objects.None, nil
}

func sysStreamRepr(args []objects.Object) (objects.Object, error) {
	name, err := objects.LoadAttr(args[0], "name")
	if err != nil {
		return nil, err
	}
	mode, err := objects.LoadAttr(args[0], "mode")
	if err != nil {
		return nil, err
	}
	return objects.NewStr("<_io.TextIOWrapper name='" + objects.Str(name) +
		"' mode='" + objects.Str(mode) + "' encoding='utf-8'>"), nil
}

func sysStreamWritable(args []objects.Object) (objects.Object, error) {
	return sysStreamDirection(args[0], "w"), nil
}

func sysStreamReadable(args []objects.Object) (objects.Object, error) {
	return sysStreamDirection(args[0], "r"), nil
}

// sysStreamDirection answers writable/readable by matching the stream mode, so
// stdin reads and stdout and stderr write.
func sysStreamDirection(self objects.Object, want string) objects.Object {
	mode, err := objects.LoadAttr(self, "mode")
	if err == nil {
		if m, ok := objects.AsStr(mode); ok && m == want {
			return objects.True
		}
	}
	return objects.False
}

func sysStreamFalse(args []objects.Object) (objects.Object, error) {
	return objects.False, nil
}

func sysStreamFileno(args []objects.Object) (objects.Object, error) {
	fd, err := objects.LoadAttr(args[0], "__fd__")
	if err != nil {
		return nil, err
	}
	return fd, nil
}
