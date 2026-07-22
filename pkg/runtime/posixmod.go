package runtime

import (
	"os"
	"strings"
	"syscall"

	"github.com/tamnd/unagi/pkg/objects"
)

// posix is the syscall accelerator os.py runs on: `from posix import *` pulls in
// the open flags, the access-mode constants and the process-and-directory calls.
// This skeleton stands up the leaf surface that needs no file descriptors: the
// constants, environ, the error alias and the fd-free calls. The stat family
// (6d) and the fd I/O calls (6e) land on top, and os.py itself comes up at 6f.
//
// The open flags are platform-specific (O_CREAT is 0x200 on darwin, 0x40 on
// Linux), so they come from Go's syscall constants, resolved per-GOOS at compile
// time, the way errno numbers do. The access-mode constants F_OK/R_OK/W_OK/X_OK
// are POSIX-universal (0/4/2/1 everywhere), so they are written out directly.

func init() {
	moduleTable["posix"] = &moduleEntry{builtin: true, exec: initPosix}
}

// posixOpenFlags is the open()-flag surface, taken from syscall so each host
// gets its own values. These are the flags os.py re-exports and a program
// passes to os.open; the list is the portable set both supported hosts define.
var posixOpenFlags = []struct {
	name string
	val  int
}{
	{"O_RDONLY", syscall.O_RDONLY},
	{"O_WRONLY", syscall.O_WRONLY},
	{"O_RDWR", syscall.O_RDWR},
	{"O_APPEND", syscall.O_APPEND},
	{"O_CREAT", syscall.O_CREAT},
	{"O_EXCL", syscall.O_EXCL},
	{"O_TRUNC", syscall.O_TRUNC},
	{"O_NONBLOCK", syscall.O_NONBLOCK},
	{"O_NDELAY", syscall.O_NDELAY},
	{"O_SYNC", syscall.O_SYNC},
	{"O_NOCTTY", syscall.O_NOCTTY},
	{"O_CLOEXEC", syscall.O_CLOEXEC},
	{"O_DIRECTORY", syscall.O_DIRECTORY},
	{"O_NOFOLLOW", syscall.O_NOFOLLOW},
}

func initPosix(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}

	// error is posix's spelling of OSError, the class its calls raise. os.py
	// re-exports it, and older code still catches os.error.
	if oserr, ok := objects.ExcClassValue("OSError"); ok {
		if err := set("error", oserr); err != nil {
			return err
		}
	}

	for _, f := range posixOpenFlags {
		if err := set(f.name, objects.NewInt(int64(f.val))); err != nil {
			return err
		}
	}
	// The access() mode constants are the same on every POSIX host.
	access := map[string]int64{"F_OK": 0, "R_OK": 4, "W_OK": 2, "X_OK": 1}
	for name, val := range access {
		if err := set(name, objects.NewInt(val)); err != nil {
			return err
		}
	}

	// environ is the process environment as a bytes->bytes dict, the raw form
	// posix exposes; os.py decodes it into the str-keyed os.environ on top.
	environ, err := posixEnvironDict()
	if err != nil {
		return err
	}
	if err := set("environ", environ); err != nil {
		return err
	}
	// _create_environ hands os.py a fresh snapshot at import time.
	if err := set("_create_environ", objects.NewFunc("_create_environ", 0, func(args []objects.Object) (objects.Object, error) {
		if len(args) != 0 {
			return nil, objects.Raise(objects.TypeError, "_create_environ() takes no arguments (%d given)", len(args))
		}
		return posixEnvironDict()
	})); err != nil {
		return err
	}

	// _have_functions gates the fd/dir_fd-aware call variants os.py advertises.
	// This skeleton implements none of them yet, so the list is empty and grows
	// as the fd slices (6e) land; os.py treats the empty list as "no fd support".
	if err := set("_have_functions", objects.NewList(nil)); err != nil {
		return err
	}

	// stat_result is the structseq stat/lstat/fstat return; os.py re-exports it.
	if err := set("stat_result", posixStatResultType); err != nil {
		return err
	}

	// DirEntry and the scandir iterator are Go classObjects, built once and
	// shared across imports. scandir yields DirEntry values; os.py re-exports
	// DirEntry and os.walk drives scandir.
	if posixDirEntryClass == nil {
		cls, err := buildPosixDirEntry()
		if err != nil {
			return err
		}
		posixDirEntryClass = cls
	}
	if posixScandirClass == nil {
		cls, err := buildPosixScandir()
		if err != nil {
			return err
		}
		posixScandirClass = cls
	}
	if err := set("DirEntry", posixDirEntryClass); err != nil {
		return err
	}
	if err := set("scandir", objects.NewFunc("scandir", -1, posixScandir)); err != nil {
		return err
	}

	fns := []struct {
		name string
		fn   func([]objects.Object) (objects.Object, error)
	}{
		{"getcwd", posixGetcwd},
		{"getcwdb", posixGetcwdb},
		{"getpid", posixGetpid},
		{"getppid", posixGetppid},
		{"strerror", posixStrerror},
		{"umask", posixUmask},
		{"listdir", posixListdir},
		{"stat", posixStat},
		{"lstat", posixLstat},
		{"fstat", posixFstat},
		{"access", posixAccess},
	}
	for _, f := range fns {
		if err := set(f.name, objects.NewFunc(f.name, -1, f.fn)); err != nil {
			return err
		}
	}
	return nil
}

// posixEnvironDict snapshots the process environment as a bytes->bytes dict.
func posixEnvironDict() (objects.Object, error) {
	d, err := objects.NewDict(nil, nil)
	if err != nil {
		return nil, err
	}
	for _, kv := range os.Environ() {
		if name, val, ok := strings.Cut(kv, "="); ok {
			k := objects.NewBytes([]byte(name))
			v := objects.NewBytes([]byte(val))
			if serr := objects.SetItem(d, k, v); serr != nil {
				return nil, serr
			}
		}
	}
	return d, nil
}

func posixGetcwd(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "getcwd() takes no arguments (%d given)", len(args))
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, objects.Raise("OSError", "%s", err.Error())
	}
	return objects.NewStr(wd), nil
}

func posixGetcwdb(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "getcwdb() takes no arguments (%d given)", len(args))
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, objects.Raise("OSError", "%s", err.Error())
	}
	return objects.NewBytes([]byte(wd)), nil
}

func posixGetpid(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "getpid() takes no arguments (%d given)", len(args))
	}
	return objects.NewInt(int64(os.Getpid())), nil
}

func posixGetppid(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "getppid() takes no arguments (%d given)", len(args))
	}
	return objects.NewInt(int64(os.Getppid())), nil
}

// posixStrerror maps an errno to its message. The text comes from Go's errno
// table, which is close to but not byte-identical with the host libc strerror
// CPython uses (Go lowercases the first word), so callers should not depend on
// the exact wording; it is platform-specific either way.
func posixStrerror(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "strerror() takes exactly one argument (%d given)", len(args))
	}
	code, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[0].TypeName())
	}
	return objects.NewStr(syscall.Errno(code).Error()), nil
}

// posixUmask sets the process file-mode creation mask and returns the previous
// one, the same set-and-return contract as C umask.
func posixUmask(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "umask() takes exactly one argument (%d given)", len(args))
	}
	mask, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[0].TypeName())
	}
	return objects.NewInt(int64(syscall.Umask(int(mask)))), nil
}

// posixListdir lists a directory's entries, defaulting to the current one. The
// names come back in directory order, the arbitrary order CPython's listdir
// returns, so a caller that needs a fixed order sorts them itself.
func posixListdir(args []objects.Object) (objects.Object, error) {
	if len(args) > 1 {
		return nil, objects.Raise(objects.TypeError, "listdir() takes at most 1 argument (%d given)", len(args))
	}
	dir := "."
	if len(args) == 1 && args[0] != objects.None {
		s, ok := objects.AsStr(args[0])
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
