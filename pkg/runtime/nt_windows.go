//go:build windows

package runtime

import (
	"crypto/rand"
	"os"
	"runtime"
	"strings"
	"syscall"

	"github.com/tamnd/unagi/pkg/objects"
)

// nt is the Windows analog of the posix accelerator: os.py runs `from nt import
// *` on Windows the way it does `from posix import *` on unix, so registering
// this module is what lets `import os` come up at all (os.py raises "no os
// specific module found" when neither posix nor nt is a builtin). Because
// posixmod.go is tagged !windows, only nt registers here, and it flows straight
// into sys.builtin_module_names through ShimmedModules, so os.py takes its nt
// branch.
//
// This carries the surface os.py needs to import and drive the common path
// operations on Windows: the constants, the string environment, the fd-free
// directory/file calls, the stat family (its own stat_result shape with
// st_file_attributes/st_reparse_tag, in ntstat_windows.go), scandir/DirEntry
// (ntscandir_windows.go), and the fd I/O calls over syscall.Handle
// (ntfd_windows.go). os.py's module-level fwalk guard references the stat and
// scandir names and the _have_functions block references stat, so those have to
// be present for `import os` to complete.

func init() {
	moduleTable["nt"] = &moduleEntry{builtin: true, exec: initNt}
}

// ntOpenFlags is the open()-flag surface with the MSVCRT values CPython's nt
// reports on Windows. These are the C runtime's fcntl values (O_CREAT is 0x100,
// not the 0x40 unix uses), distinct from Go's windows syscall constants, so they
// are written out directly against the CPython 3.14.6 oracle.
var ntOpenFlags = []struct {
	name string
	val  int64
}{
	{"O_RDONLY", 0},
	{"O_WRONLY", 1},
	{"O_RDWR", 2},
	{"O_APPEND", 8},
	{"O_CREAT", 256},
	{"O_TRUNC", 512},
	{"O_EXCL", 1024},
	{"O_TEXT", 16384},
	{"O_BINARY", 32768},
	{"O_NOINHERIT", 128},
	{"O_SHORT_LIVED", 4096},
	{"O_TEMPORARY", 64},
	{"O_RANDOM", 16},
	{"O_SEQUENTIAL", 32},
}

func initNt(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}

	// error is nt's spelling of OSError, the class its calls raise; os.py
	// re-exports it as os.error.
	if oserr, ok := objects.ExcClassValue("OSError"); ok {
		if err := set("error", oserr); err != nil {
			return err
		}
	}

	for _, f := range ntOpenFlags {
		if err := set(f.name, objects.NewInt(f.val)); err != nil {
			return err
		}
	}
	// The access() mode constants are the same values Windows reports.
	access := map[string]int64{"F_OK": 0, "R_OK": 4, "W_OK": 2, "X_OK": 1}
	for name, val := range access {
		if err := set(name, objects.NewInt(val)); err != nil {
			return err
		}
	}

	// environ is str->str on Windows (unlike posix's bytes->bytes), the shape
	// os.py's nt branch decodes into os.environ.
	environ, err := ntEnvironDict()
	if err != nil {
		return err
	}
	if err := set("environ", environ); err != nil {
		return err
	}
	if err := set("_create_environ", objects.NewFunc("_create_environ", 0, func(args []objects.Object) (objects.Object, error) {
		if len(args) != 0 {
			return nil, objects.Raise(objects.TypeError, "_create_environ() takes no arguments (%d given)", len(args))
		}
		return ntEnvironDict()
	})); err != nil {
		return err
	}

	// _have_functions is the capability list os.py reads to build its supports_*
	// sets. Windows CPython reports exactly these two: HAVE_FTRUNCATE and the
	// MS_WINDOWS marker. The block references the stat name (`_set.add(stat)`),
	// which the stat family below provides, so exporting the list is safe.
	haveFns := []objects.Object{objects.NewStr("HAVE_FTRUNCATE"), objects.NewStr("MS_WINDOWS")}
	if err := set("_have_functions", objects.NewList(haveFns)); err != nil {
		return err
	}

	// stat_result is the structseq stat/lstat/fstat return; os.py re-exports it.
	if err := set("stat_result", ntStatResultType); err != nil {
		return err
	}

	// DirEntry and the scandir iterator are Go classObjects, built once and
	// shared across imports. os.py re-exports DirEntry and os.walk drives scandir.
	if ntDirEntryClass == nil {
		cls, err := buildNtDirEntry()
		if err != nil {
			return err
		}
		ntDirEntryClass = cls
	}
	if ntScandirClass == nil {
		cls, err := buildNtScandir()
		if err != nil {
			return err
		}
		ntScandirClass = cls
	}
	if err := set("DirEntry", ntDirEntryClass); err != nil {
		return err
	}
	if err := set("scandir", objects.NewFunc("scandir", -1, ntScandir)); err != nil {
		return err
	}

	fns := []struct {
		name string
		fn   func([]objects.Object) (objects.Object, error)
	}{
		{"getcwd", ntGetcwd},
		{"getcwdb", ntGetcwdb},
		{"getpid", ntGetpid},
		{"getppid", ntGetppid},
		{"strerror", ntStrerror},
		{"cpu_count", ntCPUCount},
		{"urandom", ntUrandom},
		{"listdir", ntListdir},
		{"stat", ntStat},
		{"lstat", ntLstat},
		{"fstat", ntFstat},
		{"access", ntAccess},
		{"open", ntOpen},
		{"read", ntRead},
		{"write", ntWrite},
		{"close", ntClose},
		{"lseek", ntLseek},
		{"dup", ntDup},
		{"ftruncate", ntFtruncate},
		{"isatty", ntIsatty},
		{"_exit", ntExit},
		{"unlink", ntUnlink},
		{"remove", ntUnlink},
		{"mkdir", ntMkdir},
		{"rmdir", ntRmdir},
		{"rename", ntRename},
		{"replace", ntRename},
		{"putenv", ntPutenv},
		{"unsetenv", ntUnsetenv},
		{"fspath", ntFspathFn},
	}
	for _, f := range fns {
		if err := set(f.name, objects.NewFunc(f.name, -1, f.fn)); err != nil {
			return err
		}
	}

	// __all__ gives os.py's _get_exports_list the public surface without a dir()
	// builtin, the same contract the posix module fills.
	names := m.PublicNames()
	all := make([]objects.Object, len(names))
	for i, n := range names {
		all[i] = objects.NewStr(n)
	}
	if err := set("__all__", objects.NewList(all)); err != nil {
		return err
	}
	return nil
}

// ntEnvironDict snapshots the process environment as a str->str dict, the form
// nt.environ takes on Windows.
func ntEnvironDict() (objects.Object, error) {
	d, err := objects.NewDict(nil, nil)
	if err != nil {
		return nil, err
	}
	for _, kv := range os.Environ() {
		if name, val, ok := strings.Cut(kv, "="); ok {
			// Windows env-var names are case-insensitive; CPython keeps the
			// first spelling the OS returns, so a straight snapshot matches.
			if serr := objects.SetItem(d, objects.NewStr(name), objects.NewStr(val)); serr != nil {
				return nil, serr
			}
		}
	}
	return d, nil
}

// ntFsPath reads a filesystem path argument as a string, accepting str and bytes.
func ntFsPath(o objects.Object) (string, bool) {
	if b, ok := objects.AsBytes(o); ok {
		return string(b), true
	}
	return objects.AsStr(o)
}

func ntGetcwd(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "getcwd() takes no arguments (%d given)", len(args))
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, objects.Raise("OSError", "%s", err.Error())
	}
	return objects.NewStr(wd), nil
}

func ntGetcwdb(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "getcwdb() takes no arguments (%d given)", len(args))
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, objects.Raise("OSError", "%s", err.Error())
	}
	return objects.NewBytes([]byte(wd)), nil
}

func ntGetpid(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "getpid() takes no arguments (%d given)", len(args))
	}
	return objects.NewInt(int64(os.Getpid())), nil
}

func ntGetppid(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "getppid() takes no arguments (%d given)", len(args))
	}
	return objects.NewInt(int64(os.Getppid())), nil
}

// ntStrerror maps an errno to its message via Go's errno table, close to but not
// byte-identical with the CRT strerror; the text is platform-specific either way.
func ntStrerror(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "strerror() takes exactly one argument (%d given)", len(args))
	}
	code, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[0].TypeName())
	}
	return objects.NewStr(syscall.Errno(code).Error()), nil
}

func ntCPUCount(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "cpu_count() takes no arguments (%d given)", len(args))
	}
	return objects.NewInt(int64(runtime.NumCPU())), nil
}

// ntUrandom returns size cryptographically random bytes from the OS entropy
// source, the same contract as posix.urandom.
func ntUrandom(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "urandom() takes exactly one argument (%d given)", len(args))
	}
	n, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
	}
	if n < 0 {
		return nil, objects.Raise(objects.ValueError, "negative argument not allowed")
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return nil, objects.Raise("OSError", "%s", err.Error())
	}
	return objects.NewBytes(buf), nil
}

func ntListdir(args []objects.Object) (objects.Object, error) {
	if len(args) > 1 {
		return nil, objects.Raise(objects.TypeError, "listdir() takes at most 1 argument (%d given)", len(args))
	}
	dir := "."
	if len(args) == 1 && args[0] != objects.None {
		s, ok := ntFsPath(args[0])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "listdir: path should be string, not %s", args[0].TypeName())
		}
		dir = s
	}
	f, err := os.Open(dir)
	if err != nil {
		return nil, objects.Raise("FileNotFoundError", "%s", err.Error())
	}
	defer func() { _ = f.Close() }()
	entries, err := f.Readdirnames(-1)
	if err != nil {
		return nil, objects.Raise("OSError", "%s", err.Error())
	}
	names := make([]objects.Object, len(entries))
	for i, e := range entries {
		names[i] = objects.NewStr(e)
	}
	return objects.NewList(names), nil
}

// ntAccess answers whether the process can access a path with the given mode,
// returning a bool rather than raising. Windows has no per-user permission bits
// the way POSIX does, so read/execute reduce to existence and only W_OK also
// checks the read-only attribute, matching how CPython's nt access behaves.
func ntAccess(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "access() takes exactly 2 arguments (%d given)", len(args))
	}
	p, ok := ntFsPath(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "access: path should be string, not %s", args[0].TypeName())
	}
	mode, ok := objects.AsInt(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[1].TypeName())
	}
	info, err := os.Stat(p)
	if err != nil {
		return objects.NewBool(false), nil
	}
	if mode&2 != 0 && info.Mode().Perm()&0o200 == 0 {
		// W_OK on a read-only file.
		return objects.NewBool(false), nil
	}
	return objects.NewBool(true), nil
}

func ntExit(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "_exit() takes exactly one argument (%d given)", len(args))
	}
	code, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
	}
	os.Exit(int(code))
	return objects.None, nil // unreachable
}

func ntUnlink(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "unlink() takes exactly 1 argument (%d given)", len(args))
	}
	path, ok := ntFsPath(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "unlink: path should be string or bytes, not %s", args[0].TypeName())
	}
	if info, err := os.Lstat(path); err == nil && info.IsDir() {
		return nil, objects.Raise("PermissionError", "[Errno 13] Permission denied: '%s'", path)
	}
	if err := os.Remove(path); err != nil {
		return nil, ntPathErr(err)
	}
	return objects.None, nil
}

func ntMkdir(args []objects.Object) (objects.Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, objects.Raise(objects.TypeError, "mkdir() takes at most 2 arguments (%d given)", len(args))
	}
	path, ok := ntFsPath(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "mkdir: path should be string or bytes, not %s", args[0].TypeName())
	}
	// The mode argument is accepted and ignored: Windows has no umask-masked
	// permission bits, so CPython's nt.mkdir ignores it too.
	if err := os.Mkdir(path, 0o777); err != nil {
		return nil, ntPathErr(err)
	}
	return objects.None, nil
}

func ntRmdir(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "rmdir() takes exactly 1 argument (%d given)", len(args))
	}
	path, ok := ntFsPath(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "rmdir: path should be string or bytes, not %s", args[0].TypeName())
	}
	// rmdir only removes a directory: a non-directory raises rather than being
	// unlinked, matching CPython.
	if info, err := os.Lstat(path); err == nil && !info.IsDir() {
		return nil, objects.Raise("NotADirectoryError", "[Errno 20] Not a directory: '%s'", path)
	}
	if err := os.Remove(path); err != nil {
		return nil, ntPathErr(err)
	}
	return objects.None, nil
}

func ntRename(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "rename() takes exactly 2 arguments (%d given)", len(args))
	}
	src, ok := ntFsPath(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "rename: src should be string or bytes, not %s", args[0].TypeName())
	}
	dst, ok := ntFsPath(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "rename: dst should be string or bytes, not %s", args[1].TypeName())
	}
	if err := os.Rename(src, dst); err != nil {
		return nil, ntPathErr(err)
	}
	return objects.None, nil
}

func ntPutenv(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "putenv() takes exactly 2 arguments (%d given)", len(args))
	}
	key, ok := ntFsPath(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "putenv() argument 1 must be str or bytes, not %s", args[0].TypeName())
	}
	val, ok := ntFsPath(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "putenv() argument 2 must be str or bytes, not %s", args[1].TypeName())
	}
	if strings.ContainsRune(key, '=') {
		return nil, objects.Raise(objects.ValueError, "illegal environment variable name")
	}
	if err := os.Setenv(key, val); err != nil {
		return nil, ntPathErr(err)
	}
	return objects.None, nil
}

func ntUnsetenv(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "unsetenv() takes exactly 1 argument (%d given)", len(args))
	}
	key, ok := ntFsPath(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "unsetenv() argument 1 must be str or bytes, not %s", args[0].TypeName())
	}
	if err := os.Unsetenv(key); err != nil {
		return nil, ntPathErr(err)
	}
	return objects.None, nil
}

// ntFspathFn is nt.fspath(path): it returns the str or bytes path unchanged, or
// calls __fspath__ on a path-like object, the low-level primitive os.fspath and
// the PathLike protocol build on.
func ntFspathFn(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "fspath() takes exactly 1 argument (%d given)", len(args))
	}
	arg := args[0]
	if _, ok := objects.AsStr(arg); ok {
		return arg, nil
	}
	if _, ok := objects.AsBytes(arg); ok {
		return arg, nil
	}
	m, err := objects.LoadAttr(arg, "__fspath__")
	if err != nil {
		return nil, objects.Raise(objects.TypeError, "expected str, bytes or os.PathLike object, not %s", arg.TypeName())
	}
	r, err := objects.Call(m, nil)
	if err != nil {
		return nil, err
	}
	if _, ok := objects.AsStr(r); ok {
		return r, nil
	}
	if _, ok := objects.AsBytes(r); ok {
		return r, nil
	}
	return nil, objects.Raise(objects.TypeError, "expected __fspath__() to return str or bytes, not %s", r.TypeName())
}

// ntPathErr maps a Go filesystem error to the matching Python exception, the
// FileNotFoundError / PermissionError special cases plus plain OSError.
func ntPathErr(err error) error {
	switch {
	case os.IsNotExist(err):
		return objects.Raise("FileNotFoundError", "%s", err.Error())
	case os.IsPermission(err):
		return objects.Raise("PermissionError", "%s", err.Error())
	case os.IsExist(err):
		return objects.Raise("FileExistsError", "%s", err.Error())
	}
	return objects.Raise("OSError", "%s", err.Error())
}
