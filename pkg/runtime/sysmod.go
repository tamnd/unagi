package runtime

import (
	"math"
	"runtime"
	"strings"
	"sync"

	"github.com/tamnd/unagi/pkg/objects"
)

// sysPlatform reports sys.platform for the host the compiled program runs on,
// mapping Go's GOOS to the value CPython uses: darwin and linux pass through,
// and windows reads as win32, the string the stdlib branches on.
func sysPlatform() string {
	switch runtime.GOOS {
	case "windows":
		return "win32"
	default:
		return runtime.GOOS
	}
}

// sys is the first built-in module: the runtime registers it in the import
// table itself, so `import sys` works in every compiled program without a
// table entry from the build. sys.modules is the live dict the import
// machinery reads and writes, not a copy, which is what makes pokes, deletes,
// None entries, and sys.modules[__name__] = obj self-replacement take effect.
//
// The identity attributes below carry the pinned CPython's own values, so a
// floor module that gates on sys.version_info or reads sys.maxsize sees what it
// would under CPython 3.14.6. version_info is a plain tuple here rather than the
// struct sequence CPython uses: the >= and indexing a version gate needs behave
// identically, but the tm-style named fields (.major) and the
// sys.version_info(...) repr are an accepted divergence until the struct
// sequence type lands.

// The pinned oracle, mirrored from conformance/ORACLE_PIN. Moving the pin moves
// these in the same diff, so a version bump stays visible in review.
const (
	pyMajor        = 3
	pyMinor        = 14
	pyMicro        = 6
	pyReleaseLevel = "final"
	pySerial       = 0
)

func init() {
	moduleTable["sys"] = &moduleEntry{builtin: true, exec: initSys}
}

// The thread switch interval, in seconds, that sys.getswitchinterval reads back
// and sys.setswitchinterval stores. CPython uses it to pace how often the
// interpreter yields the GIL; compiled programs run on Go's own scheduler, so
// the value is a functional no-op kept only so a program that reads or tunes it
// sees the value it set. The mutex keeps the getter and setter race-clean when
// threads touch it at once. The default matches CPython's 5ms.
var (
	switchIntervalMu sync.Mutex
	switchInterval   = 0.005
)

// sysGetRecursionLimit implements sys.getrecursionlimit(): the current recursion
// limit, the same process-wide value the frame-depth guard in recursion.go charges
// against.
func sysGetRecursionLimit(args []objects.Object) (objects.Object, error) {
	return objects.NewInt(int64(RecursionLimit())), nil
}

// sysSetRecursionLimit implements sys.setrecursionlimit(n): set the process-wide
// recursion limit. The argument must read as an integer, a non-integer is the
// TypeError CPython raises coercing it, and a limit below one is the ValueError.
// CPython also raises RecursionError at set time when the new limit is at or below
// the current depth; that check is deferred here because the boxed frame depth does
// not line up with CPython's frame count and its "at the recursion depth N" message
// is not byte-matchable, so a too-low limit is enforced lazily at the next frame
// charge instead.
func sysSetRecursionLimit(args []objects.Object) (objects.Object, error) {
	n, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
	}
	if n < 1 {
		return nil, objects.Raise(objects.ValueError, "recursion limit must be greater or equal than 1")
	}
	SetRecursionLimit(int(n))
	return objects.None, nil
}

// sysGetSwitchInterval implements sys.getswitchinterval(): the current interval
// as a float, 0.005 until a program sets its own.
func sysGetSwitchInterval(args []objects.Object) (objects.Object, error) {
	switchIntervalMu.Lock()
	v := switchInterval
	switchIntervalMu.Unlock()
	return objects.NewFloat(v), nil
}

// sysSetSwitchInterval implements sys.setswitchinterval(n): store a strictly
// positive interval. A non-number is the TypeError CPython raises coercing the
// argument to a float, and a value that is zero or negative is the ValueError.
func sysSetSwitchInterval(args []objects.Object) (objects.Object, error) {
	n, ok := objects.AsFloat(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "must be real number, not %s", args[0].TypeName())
	}
	if !(n > 0) {
		return nil, objects.Raise(objects.ValueError, "switch interval must be strictly positive")
	}
	switchIntervalMu.Lock()
	switchInterval = n
	switchIntervalMu.Unlock()
	return objects.None, nil
}

func initSys(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	if err := set("modules", modules); err != nil {
		return err
	}
	versionInfo := objects.NewTuple([]objects.Object{
		objects.NewInt(pyMajor),
		objects.NewInt(pyMinor),
		objects.NewInt(pyMicro),
		objects.NewStr(pyReleaseLevel),
		objects.NewInt(pySerial),
	})
	// hexversion packs the version the way CPython's PY_VERSION_HEX macro does:
	// major, minor, micro in a byte each, then the release level nibble (final is
	// 0xF) and the serial nibble.
	hex := int64(pyMajor)<<24 | int64(pyMinor)<<16 | int64(pyMicro)<<8 | 0xF0 | int64(pySerial)
	attrs := []struct {
		name string
		val  objects.Object
	}{
		{"version_info", versionInfo},
		{"hexversion", objects.NewInt(hex)},
		{"maxsize", objects.NewInt(math.MaxInt64)},
		{"maxunicode", objects.NewInt(0x10FFFF)},
		{"byteorder", objects.NewStr("little")},
		{"platform", objects.NewStr(sysPlatform())},
	}
	for _, a := range attrs {
		if err := set(a.name, a.val); err != nil {
			return err
		}
	}
	if err := set("getswitchinterval", objects.NewFunc("getswitchinterval", 0, sysGetSwitchInterval)); err != nil {
		return err
	}
	if err := set("setswitchinterval", objects.NewFunc("setswitchinterval", 1, sysSetSwitchInterval)); err != nil {
		return err
	}
	if err := set("getrecursionlimit", objects.NewFunc("getrecursionlimit", 0, sysGetRecursionLimit)); err != nil {
		return err
	}
	if err := set("setrecursionlimit", objects.NewFunc("setrecursionlimit", 1, sysSetRecursionLimit)); err != nil {
		return err
	}
	if err := set("_getframe", objects.NewFuncT("_getframe", -1, sysGetFrame)); err != nil {
		return err
	}
	if err := set("builtin_module_names", sysBuiltinModuleNames()); err != nil {
		return err
	}
	if err := set("getfilesystemencoding", objects.NewFunc("getfilesystemencoding", 0, sysGetFilesystemEncoding)); err != nil {
		return err
	}
	if err := set("getfilesystemencodeerrors", objects.NewFunc("getfilesystemencodeerrors", 0, sysGetFilesystemEncodeErrors)); err != nil {
		return err
	}
	return nil
}

// sysBuiltinModuleNames builds sys.builtin_module_names: the sorted tuple of
// statically linked module names. Every Go-shimmed module is the analog of a
// CPython C builtin, so the source is ShimmedModules(). Dotted names like
// os.path are dropped since CPython lists only top-level modules there; os.py
// only tests membership of posix, so an honest top-level set is enough.
func sysBuiltinModuleNames() objects.Object {
	names := ShimmedModules()
	elts := make([]objects.Object, 0, len(names))
	for _, n := range names {
		if strings.Contains(n, ".") {
			continue
		}
		elts = append(elts, objects.NewStr(n))
	}
	return objects.NewTuple(elts)
}

// sysGetFilesystemEncoding reports sys.getfilesystemencoding(). Since 3.7 the
// filesystem encoding is always UTF-8, the value os.py's fsencode/fsdecode and
// _fscodec build on.
func sysGetFilesystemEncoding(args []objects.Object) (objects.Object, error) {
	return objects.NewStr("utf-8"), nil
}

// sysGetFilesystemEncodeErrors reports sys.getfilesystemencodeerrors(): the
// error handler paired with the filesystem encoding, surrogateescape on POSIX.
func sysGetFilesystemEncodeErrors(args []objects.Object) (objects.Object, error) {
	return objects.NewStr("surrogateescape"), nil
}

// sysGetFrame implements sys._getframe(depth=0): return the frame depth levels
// above the caller of _getframe, the entry point _collections_abc and traceback
// machinery reach for. It takes the ambient thread so it reads that thread's own
// shadow stack. depth defaults to 0, must read as an integer, and a depth past
// the bottom of the stack is the ValueError CPython raises. _getframe is a
// builtin and pushes no frame of its own, so depth 0 is the compiled Python
// function that called it.
func sysGetFrame(t *objects.Thread, args []objects.Object) (objects.Object, error) {
	if len(args) > 1 {
		return nil, objects.Raise(objects.TypeError, "_getframe expected at most 1 argument, got %d", len(args))
	}
	depth := 0
	if len(args) == 1 {
		n, ok := objects.AsInt(args[0])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
		}
		depth = int(n)
	}
	return t.FrameAtDepth(depth)
}
