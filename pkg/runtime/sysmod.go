package runtime

import (
	"fmt"
	"math"
	"os"
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

// sysArgv builds sys.argv from the process arguments. A compiled program is a
// native executable, so os.Args carries the argument vector directly: argv[0]
// is the program path and argv[1:] the arguments. The result is a fresh mutable
// list so a program may rewrite sys.argv in place.
func sysArgv() objects.Object {
	argv := make([]objects.Object, len(os.Args))
	for i, a := range os.Args {
		argv[i] = objects.NewStr(a)
	}
	return objects.NewList(argv)
}

// sys is the first built-in module: the runtime registers it in the import
// table itself, so `import sys` works in every compiled program without a
// table entry from the build. sys.modules is the live dict the import
// machinery reads and writes, not a copy, which is what makes pokes, deletes,
// None entries, and sys.modules[__name__] = obj self-replacement take effect.
//
// The identity attributes below carry the pinned CPython's own values, so a
// floor module that gates on sys.version_info or reads sys.maxsize sees what it
// would under CPython 3.14.6. version_info is a struct sequence like CPython's,
// so a >= or indexing gate, the named fields (.major, .releaselevel), and the
// sys.version_info(...) repr all behave the way stdlib code expects.

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

// sysVersionInfoType is the struct sequence behind sys.version_info. All five
// fields are the visible sequence CPython exposes, so it indexes and compares as
// a tuple while also answering .major, .minor, .micro, .releaselevel and
// .serial, and reprs as sys.version_info(major=3, minor=14, ...).
var sysVersionInfoType = objects.NewStructSeqType(
	"sys.version_info", "sys.version_info",
	[]string{"major", "minor", "micro", "releaselevel", "serial"},
	5, 0,
)

// sysVersionInfo builds sys.version_info from the pinned oracle constants.
func sysVersionInfo() objects.Object {
	seq := []objects.Object{
		objects.NewInt(pyMajor),
		objects.NewInt(pyMinor),
		objects.NewInt(pyMicro),
		objects.NewStr(pyReleaseLevel),
		objects.NewInt(pySerial),
	}
	return sysVersionInfoType.NewStructSeq(seq, seq)
}

// sysMultiarch reports sys.implementation._multiarch, the platform tag sysconfig
// joins into its config paths. CPython leaves it 'darwin' on macOS and the GNU
// triplet on Linux; it is derived from the host so a compiled program reports the
// tag of the machine it runs on, and code that only reads it for a path prefix
// gets a plausible value.
func sysMultiarch() string {
	switch runtime.GOOS {
	case "darwin":
		return "darwin"
	case "linux":
		arch := runtime.GOARCH
		switch arch {
		case "amd64":
			arch = "x86_64"
		case "arm64":
			arch = "aarch64"
		}
		return arch + "-linux-gnu"
	default:
		return runtime.GOARCH + "-" + runtime.GOOS
	}
}

// sysImplementation builds sys.implementation, the SimpleNamespace describing the
// running interpreter. A compiled program executes CPython 3.14.6 semantics, so
// it reports name 'cpython' and the matching cache_tag, version and hexversion,
// which is what lets sysconfig build its cache paths and import (unblocking
// zoneinfo through it). It is a genuine types.SimpleNamespace so a read of any
// attribute and its repr match what stdlib code reads off the real one.
func sysImplementation(hex int64) objects.Object {
	return objects.NewSimpleNamespace(
		[]string{"name", "cache_tag", "version", "hexversion", "_multiarch", "supports_isolated_interpreters"},
		[]objects.Object{
			objects.NewStr("cpython"),
			objects.NewStr(fmt.Sprintf("cpython-%d%d", pyMajor, pyMinor)),
			sysVersionInfo(),
			objects.NewInt(hex),
			objects.NewStr(sysMultiarch()),
			objects.False,
		},
	)
}

// sysFlagsType is the structseq class behind sys.flags. The first 18 fields are
// the visible sequence CPython 3.14 exposes; gil, thread_inherit_context and
// context_aware_warnings are named-only, past n_sequence_fields, the way the
// stat_result extras are. n_unnamed_fields is 0.
var sysFlagsType = objects.NewStructSeqType(
	"sys.flags", "sys.flags",
	[]string{
		"debug", "inspect", "interactive", "optimize", "dont_write_bytecode",
		"no_user_site", "no_site", "ignore_environment", "verbose",
		"bytes_warning", "quiet", "hash_randomization", "isolated", "dev_mode",
		"utf8_mode", "warn_default_encoding", "safe_path", "int_max_str_digits",
		"gil", "thread_inherit_context", "context_aware_warnings",
	},
	18, 0,
)

// sysHashInfoType is the structseq class behind sys.hash_info, the parameters of
// the numeric hash. All nine fields are the visible sequence CPython 3.14
// exposes; there are no named-only extras.
var sysHashInfoType = objects.NewStructSeqType(
	"sys.hash_info", "sys.hash_info",
	[]string{
		"width", "modulus", "inf", "nan", "imag",
		"algorithm", "hash_bits", "seed_bits", "cutoff",
	},
	9, 0,
)

// sysHashInfo builds sys.hash_info. The values are the ones CPython's pyhash.c
// fixes for a 64-bit build with the default siphash13 string algorithm, identical
// on every 64-bit host so the fixtures stay platform-stable. modulus is the
// Mersenne prime 2**61-1 that the numeric __hash__ of int, float and Fraction
// reduce against, which is why fractions needs this attribute to import.
func sysHashInfo() objects.Object {
	seq := []objects.Object{
		objects.NewInt(64),
		objects.NewInt((1 << 61) - 1),
		objects.NewInt(314159),
		objects.NewInt(0),
		objects.NewInt(1000003),
		objects.NewStr("siphash13"),
		objects.NewInt(64),
		objects.NewInt(128),
		objects.NewInt(0),
	}
	return sysHashInfoType.NewStructSeq(seq, seq)
}

// sysFloatInfoType is the structseq class behind sys.float_info, the limits of
// the C double every Python float rides on. All eleven fields are the visible
// sequence CPython 3.14 exposes, no named-only extras.
var sysFloatInfoType = objects.NewStructSeqType(
	"float_info", "sys.float_info",
	[]string{
		"max", "max_exp", "max_10_exp",
		"min", "min_exp", "min_10_exp",
		"dig", "mant_dig", "epsilon", "radix", "rounds",
	},
	11, 0,
)

// sysFloatInfo builds sys.float_info. The values are the IEEE 754 binary64
// limits from float.h, identical on every host that uses a 64-bit double, so the
// fixtures stay platform-stable. statistics reads mant_dig at import to size its
// exact-sum accumulator. max, min and epsilon come from exact hex-float literals
// so they carry CPython's repr to the last digit; rounds is 1 for the standard
// round-to-nearest FLT_ROUNDS.
func sysFloatInfo() objects.Object {
	seq := []objects.Object{
		objects.NewFloat(math.MaxFloat64),
		objects.NewInt(1024),
		objects.NewInt(308),
		objects.NewFloat(0x1p-1022),
		objects.NewInt(-1021),
		objects.NewInt(-307),
		objects.NewInt(15),
		objects.NewInt(53),
		objects.NewFloat(0x1p-52),
		objects.NewInt(2),
		objects.NewInt(1),
	}
	return sysFloatInfoType.NewStructSeq(seq, seq)
}

// sysFlags builds the sys.flags value. A compiled program runs none of the
// interpreter command-line switches these report, so they carry CPython's
// default startup values, hardcoded rather than read from a live interpreter
// state and identical on every host so the fixtures stay platform-stable.
// dev_mode and safe_path are bools; gil and int_max_str_digits keep CPython's
// defaults (1 and 4300); the rest are 0. hash_randomization is 0 rather than
// CPython's usual 1 because the conformance oracle runs with PYTHONHASHSEED=0,
// which disables it, and a compiled program has no hash-seed switch anyway.
func sysFlags() objects.Object {
	zero := objects.NewInt(0)
	seq := []objects.Object{
		zero, zero, zero, zero, zero,
		zero, zero, zero, zero,
		zero, zero, zero, zero, objects.False,
		zero, zero, objects.False, objects.NewInt(4300),
	}
	named := append(append([]objects.Object{}, seq...),
		objects.NewInt(1), zero, zero)
	return sysFlagsType.NewStructSeq(seq, named)
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

// sysAudit is sys.audit(event, *args): it raises an auditing event to every
// installed hook. No hooks run in this tier, so with nothing listening it is a
// no-op that returns None, which is exactly what CPython does when no hook is
// registered. The stdlib sprinkles sys.audit through tempfile, os and others,
// so it has to exist for those to run. The event name is required and must be a
// str, matching CPython's own check.
func sysAudit(args []objects.Object) (objects.Object, error) {
	if len(args) < 1 {
		return nil, objects.Raise(objects.TypeError, "audit() missing 1 required positional argument: 'event'")
	}
	if _, ok := objects.AsStr(args[0]); !ok {
		return nil, objects.Raise(objects.TypeError, "expected str for argument 'event', not %s", args[0].TypeName())
	}
	return objects.None, nil
}

// sysAddAuditHook is sys.addaudithook(hook): it registers a callable to receive
// auditing events. This tier raises no events, so the hook would never fire;
// the call still checks the argument is callable the way CPython does and then
// keeps nothing, since there is nothing to deliver to it.
func sysAddAuditHook(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "addaudithook() takes exactly one argument (%d given)", len(args))
	}
	if !objects.Callable(args[0]) {
		return nil, objects.Raise(objects.TypeError, "expected callable, not %s", args[0].TypeName())
	}
	return objects.None, nil
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
	// hexversion packs the version the way CPython's PY_VERSION_HEX macro does:
	// major, minor, micro in a byte each, then the release level nibble (final is
	// 0xF) and the serial nibble.
	hex := int64(pyMajor)<<24 | int64(pyMinor)<<16 | int64(pyMicro)<<8 | 0xF0 | int64(pySerial)
	attrs := []struct {
		name string
		val  objects.Object
	}{
		{"version_info", sysVersionInfo()},
		// sys.implementation is the SimpleNamespace naming the interpreter, its
		// cache tag and version. sysconfig reads cache_tag to build its paths, so
		// its absence stopped sysconfig (and zoneinfo through it) from importing.
		{"implementation", sysImplementation(hex)},
		// sys.version is the human-readable banner. A compiled program is not a
		// live interpreter, so the build fields carry a fixed unagi tag rather than
		// a real build date. It is shaped the way platform's CPython parser expects,
		// "<version> (<buildno>, <builddate>, <buildtime>) [<compiler>]", so
		// platform.python_version reads 3.14.6 back off it while the build and
		// compiler fields, which are host-specific in real CPython, stay a constant.
		{"version", objects.NewStr(fmt.Sprintf("%d.%d.%d (unagi, Jan 01 2026, 00:00:00) [unagi]", pyMajor, pyMinor, pyMicro))},
		{"hexversion", objects.NewInt(hex)},
		{"maxsize", objects.NewInt(math.MaxInt64)},
		{"maxunicode", objects.NewInt(0x10FFFF)},
		{"byteorder", objects.NewStr("little")},
		{"platform", objects.NewStr(sysPlatform())},
		{"flags", sysFlags()},
		{"hash_info", sysHashInfo()},
		{"float_info", sysFloatInfo()},
		{"warnoptions", objects.NewList(nil)},
		// sys.argv is the command-line argument list. A compiled program is
		// launched as a native executable, so os.Args maps straight across:
		// argv[0] is the program's own path and argv[1:] are the arguments it
		// was invoked with, the way a frozen CPython build reports them. It is a
		// mutable list so argparse and getopt can consume it and code that
		// rewrites sys.argv works.
		{"argv", sysArgv()},
		// sys.orig_argv is the argument vector as the program was launched,
		// before anything rewrites sys.argv. A compiled program has no
		// interpreter prefix, so it is the same os.Args, in a separate list so
		// editing sys.argv leaves orig_argv untouched the way CPython keeps them
		// independent.
		{"orig_argv", sysArgv()},
		// sys.path is the import search path. A compiled program resolves its
		// imports at build time, so the path plays no role in finding modules, but
		// stdlib code iterates it: linecache walks it to locate a source file and
		// warnings.warn_explicit reaches linecache through it. It is a mutable list
		// seeded with "" as path[0], CPython's convention for the current directory
		// when there is no script file, so an iteration succeeds and code that
		// prepends to sys.path works.
		{"path", objects.NewList([]objects.Object{objects.NewStr("")})},
		// A compiled program has no Python installation tree, so the install
		// prefixes are empty. They are equal to each other, which is how a program
		// tells it is not running inside a virtual environment. Modules that build
		// a default path from them, such as gettext joining a locale directory onto
		// base_prefix, just get a relative path that does not resolve.
		{"prefix", objects.NewStr("")},
		{"exec_prefix", objects.NewStr("")},
		{"base_prefix", objects.NewStr("")},
		{"base_exec_prefix", objects.NewStr("")},
		// A compiled program is not launched through a Python interpreter, so there
		// is no executable path to report. platform reads it to guess the build, and
		// with it empty falls through to the values it derives from sys itself.
		{"executable", objects.NewStr("")},
		// _framework is the macOS framework name a framework build reports, empty
		// on a non-framework build. sysconfig reads it on darwin to place the user
		// base directory, so its absence stopped sysconfig from importing.
		{"_framework", objects.NewStr("")},
		// abiflags is the ABI flag suffix on the executable name, empty on a normal
		// build. sysconfig joins it into config paths.
		{"abiflags", objects.NewStr("")},
		// copyright is the interpreter's copyright banner, the same fixed text
		// CPython carries. site reads it to build the copyright() builtin.
		{"copyright", objects.NewStr("Copyright (c) 2001 Python Software Foundation.\n" +
			"All Rights Reserved.\n\n" +
			"Copyright (c) 2000 BeOpen.com.\n" +
			"All Rights Reserved.\n\n" +
			"Copyright (c) 1995-2001 Corporation for National Research Initiatives.\n" +
			"All Rights Reserved.\n\n" +
			"Copyright (c) 1991-1995 Stichting Mathematisch Centrum, Amsterdam.\n" +
			"All Rights Reserved.")},
		// dont_write_bytecode carries CPython's default: bytecode writing is on, so
		// the flag reads False. A compiled program imports nothing at run time, so
		// the value is inert; it exists so code that branches on it runs.
		{"dont_write_bytecode", objects.NewBool(false)},
		// pycache_prefix is the directory bytecode caches are redirected to, None by
		// default. Inert here for the same reason, present so a read succeeds.
		{"pycache_prefix", objects.None},
		// The import-machinery registries. A compiled program resolves imports in
		// the native importer, not through these, so path_hooks and the importer
		// cache carry the empty forms CPython starts them at. meta_path is seeded
		// with the native finder so importlib.import_module and the rest of the pure
		// machinery resolve the same names the import statement does.
		{"path_hooks", objects.NewList(nil)},
		{"meta_path", objects.NewList([]objects.Object{nativeMetaFinder()})},
	}
	pathImporterCache, err := objects.NewDict(nil, nil)
	if err != nil {
		return err
	}
	attrs = append(attrs, struct {
		name string
		val  objects.Object
	}{"path_importer_cache", pathImporterCache})
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
	if err := set("audit", objects.NewFunc("audit", -1, sysAudit)); err != nil {
		return err
	}
	if err := set("addaudithook", objects.NewFunc("addaudithook", 1, sysAddAuditHook)); err != nil {
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
	for _, f := range []struct {
		name  string
		arity int
		fn    func(args []objects.Object) (objects.Object, error)
	}{
		{"intern", 1, sysIntern},
		{"getdefaultencoding", 0, sysGetDefaultEncoding},
		{"is_finalizing", 0, sysIsFinalizing},
		{"getallocatedblocks", 0, sysGetAllocatedBlocks},
		{"_clear_type_cache", 0, sysClearTypeCache},
		{"settrace", 1, sysSetTrace},
		{"gettrace", 0, sysGetTrace},
		{"setprofile", 1, sysSetProfile},
		{"getprofile", 0, sysGetProfile},
		{"call_tracing", 2, sysCallTracing},
		{"exception", 0, sysException},
		{"exc_info", 0, sysExcInfo},
	} {
		if err := set(f.name, objects.NewFunc(f.name, f.arity, f.fn)); err != nil {
			return err
		}
	}
	stdout, stderr, stdin, err := buildSysStreams()
	if err != nil {
		return err
	}
	for _, s := range []struct {
		name string
		v    objects.Object
	}{
		{"stdout", stdout},
		{"stderr", stderr},
		{"stdin", stdin},
		// The __stdxxx__ aliases hold the original streams so code that redirects
		// sys.stdout can restore it, matching CPython.
		{"__stdout__", stdout},
		{"__stderr__", stderr},
		{"__stdin__", stdin},
	} {
		if err := set(s.name, s.v); err != nil {
			return err
		}
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

// sysIntern implements sys.intern(string): CPython hands back a canonical copy
// so equal literals can share storage. A compiled program keeps no intern pool,
// so this returns the argument unchanged, which preserves the two properties a
// caller relies on, the result equals the argument and is a str. The argument
// must be a str, the TypeError CPython raises for anything else.
func sysIntern(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "intern() takes exactly one argument (%d given)", len(args))
	}
	if _, ok := objects.AsStr(args[0]); !ok {
		return nil, objects.Raise(objects.TypeError, "intern() argument 1 must be str, not %s", args[0].TypeName())
	}
	return args[0], nil
}

// sysGetDefaultEncoding reports sys.getdefaultencoding(): utf-8, the value
// CPython 3 fixes it at. codecs and io read it to pick a default text codec.
// sysException is sys.exception(): the exception instance currently being
// handled, or None outside an except block. It is the value half of exc_info(),
// which contextlib.ExitStack.__exit__ reads to tell whether it is unwinding an
// exception. It reads the same handled-exception stack bare `raise` and implicit
// context chaining use.
func sysException(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "exception() takes no arguments (%d given)", len(args))
	}
	if e := objects.CurrentHandled(); e != nil {
		return e, nil
	}
	return objects.None, nil
}

func sysGetDefaultEncoding(args []objects.Object) (objects.Object, error) {
	return objects.NewStr("utf-8"), nil
}

// sysExcInfo is sys.exc_info(): the (type, value, traceback) triple for the
// exception now being handled, or (None, None, None) outside an except block.
// It reads the same handled-exception stack sys.exception() and bare raise use,
// so the value is that exception and the type is its class. unagi models no
// first-class traceback object, so the third slot is None, the documented
// stand-in exc.__traceback__ also returns; unittest's testPartExecutor reads
// exc_info() to record a failure, and traceback.format_exception tolerates a
// None traceback by printing just the exception line.
func sysExcInfo(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "exc_info() takes no arguments (%d given)", len(args))
	}
	e := objects.CurrentHandled()
	if e == nil {
		return objects.NewTuple([]objects.Object{objects.None, objects.None, objects.None}), nil
	}
	var typ objects.Object = objects.ExcType(e.Kind)
	if c, ok := objects.ClassOf(e); ok {
		typ = c
	}
	return objects.NewTuple([]objects.Object{typ, e, objects.None}), nil
}

// sysIsFinalizing reports sys.is_finalizing(): whether the interpreter is
// shutting down. A compiled program that reaches user code is always running,
// never finalizing, so this is False. threading and logging read it to decide
// whether it is safe to spawn work during shutdown.
func sysIsFinalizing(args []objects.Object) (objects.Object, error) {
	return objects.NewBool(false), nil
}

// sysGetAllocatedBlocks reports sys.getallocatedblocks(): the number of memory
// blocks the allocator currently holds. Go manages memory itself and exposes no
// such count, so this reports 0. The value drives CPython leak checks that a
// compiled program does not run; code that only reads it keeps working.
func sysGetAllocatedBlocks(args []objects.Object) (objects.Object, error) {
	return objects.NewInt(0), nil
}

// sysClearTypeCache implements sys._clear_type_cache(): CPython flushes its
// internal method-resolution cache and returns None. There is no such cache to
// flush here, so this is a no-op that returns None, which is all a caller sees.
func sysClearTypeCache(args []objects.Object) (objects.Object, error) {
	return objects.None, nil
}

// Tracing and profiling hooks. CPython keeps one of each per thread and fires
// them from the bytecode eval loop. A compiled program runs native Go with no
// such loop, so a hook installed here never fires; it is stored and handed back
// so code that saves, replaces, and restores the hook, the pattern pdb, profile,
// trace and many context managers use, runs without raising. The store is
// process-wide rather than per-thread, an accepted divergence while the hooks
// stay inert.
var (
	traceHookMu sync.Mutex
	traceHook   objects.Object = objects.None
	profileHook objects.Object = objects.None
)

// sysSetTrace implements sys.settrace(function): install the trace hook, or
// clear it when passed None. The hook never fires in this tier.
func sysSetTrace(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "settrace() takes exactly one argument (%d given)", len(args))
	}
	traceHookMu.Lock()
	traceHook = args[0]
	traceHookMu.Unlock()
	return objects.None, nil
}

// sysGetTrace implements sys.gettrace(): the installed trace hook, or None when
// none is set, which is the state a program starts in.
func sysGetTrace(args []objects.Object) (objects.Object, error) {
	traceHookMu.Lock()
	v := traceHook
	traceHookMu.Unlock()
	return v, nil
}

// sysSetProfile implements sys.setprofile(function): install the profile hook,
// or clear it with None. Like the trace hook it never fires here.
func sysSetProfile(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "setprofile() takes exactly one argument (%d given)", len(args))
	}
	traceHookMu.Lock()
	profileHook = args[0]
	traceHookMu.Unlock()
	return objects.None, nil
}

// sysGetProfile implements sys.getprofile(): the installed profile hook, or None.
func sysGetProfile(args []objects.Object) (objects.Object, error) {
	traceHookMu.Lock()
	v := profileHook
	traceHookMu.Unlock()
	return v, nil
}

// sysCallTracing implements sys.call_tracing(func, args): CPython enables
// tracing and calls func(*args) for its side effects, returning None. Tracing is
// inert here, so this simply calls func(*args) and returns None. args must be a
// tuple, the type CPython requires.
func sysCallTracing(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "call_tracing() takes exactly 2 arguments (%d given)", len(args))
	}
	if args[1].TypeName() != "tuple" {
		return nil, objects.Raise(objects.TypeError, "call_tracing() argument 2 must be tuple, not %s", args[1].TypeName())
	}
	if _, err := objects.CallStarEx(args[0], args[1], nil); err != nil {
		return nil, err
	}
	return objects.None, nil
}
