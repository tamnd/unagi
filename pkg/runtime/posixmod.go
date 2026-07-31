//go:build !windows

package runtime

import (
	"crypto/rand"
	"os"
	"runtime"
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

	// terminal_size is the structseq get_terminal_size returns and
	// shutil.get_terminal_size constructs from its fallback pair.
	if err := set("terminal_size", posixTerminalSizeType); err != nil {
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
		{"_exit", posixMExit},
		{"open", posixOpen},
		{"read", posixRead},
		{"write", posixWrite},
		{"close", posixClose},
		{"lseek", posixLseek},
		{"dup", posixDup},
		{"dup2", posixDup2},
		{"pipe", posixPipe},
		{"ftruncate", posixFtruncate},
		{"fsync", posixFsync},
		{"isatty", posixIsatty},
		{"get_terminal_size", posixGetTerminalSize},
		{"cpu_count", posixCPUCount},
		{"readlink", posixReadlink},
		{"symlink", posixSymlink},
		{"getuid", posixGetuid},
		{"geteuid", posixGeteuid},
		{"putenv", posixPutenv},
		{"unsetenv", posixUnsetenv},
		{"urandom", posixUrandom},
		{"unlink", posixUnlink},
		{"remove", posixUnlink},
		{"mkdir", posixMkdir},
		{"rmdir", posixRmdir},
		{"rename", posixRename},
	}
	for _, f := range fns {
		if err := set(f.name, objects.NewFunc(f.name, -1, f.fn)); err != nil {
			return err
		}
	}
	// chmod takes the dir_fd / follow_symlinks keywords, so it is registered as a
	// keyword-aware callable rather than through the positional table above.
	if err := set("chmod", objects.NewFuncKw("chmod", posixChmod)); err != nil {
		return err
	}

	// The process-wait surface (waitpid, the W* status macros,
	// waitstatus_to_exitcode) and the fd-inheritance calls that subprocess.Popen
	// drives to launch and reap children.
	if err := initPosixProc(set); err != nil {
		return err
	}

	// __all__ gives os.py's _get_exports_list the public surface without a dir()
	// builtin: it reads posix.__all__ when present, else falls back to dir(). The
	// list is the module's own public names now that every attribute is bound.
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

// posixUrandom is os.urandom(size): it returns size cryptographically random
// bytes straight from the operating system's entropy source, the same one
// CPython draws from, so the output is not reproducible and callers seed nothing.
// A negative size is a ValueError and a non-integer a TypeError, matching CPython.
func posixUrandom(args []objects.Object) (objects.Object, error) {
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

// posixMExit is posix._exit: it ends the process now with the given status and
// skips the interpreter teardown that a normal exit runs. It does not flush
// buffered output or run cleanup, so a program that wants a line kept prints it
// with flush=True first. os.py re-exports it as os._exit.
func posixMExit(args []objects.Object) (objects.Object, error) {
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

// posixCPUCount is posix.cpu_count(): the number of CPUs the process can use.
// runtime.NumCPU is always at least one, so unlike CPython this never returns
// None; os.py re-exports it as os.cpu_count.
func posixCPUCount(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "cpu_count() takes no arguments (%d given)", len(args))
	}
	return objects.NewInt(int64(runtime.NumCPU())), nil
}

// posixReadlink is posix.readlink(path): the target a symlink points at. A str
// path returns a str target and a bytes path a bytes target, matching CPython.
// posixpath.realpath drives this to resolve link chains.
func posixReadlink(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "readlink() takes exactly 1 argument (%d given)", len(args))
	}
	if b, ok := objects.AsBytes(args[0]); ok {
		target, err := readlinkStr(string(b))
		if err != nil {
			return nil, posixStatErr(err)
		}
		return objects.NewBytes([]byte(target)), nil
	}
	p, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "readlink: path should be string or bytes, not %s", args[0].TypeName())
	}
	target, err := readlinkStr(p)
	if err != nil {
		return nil, posixStatErr(err)
	}
	return objects.NewStr(target), nil
}

// readlinkStr resolves a symlink target, growing the buffer until the whole
// target fits (a truncated read means the link is longer than the buffer).
func readlinkStr(path string) (string, error) {
	for size := 128; ; size *= 2 {
		buf := make([]byte, size)
		n, err := syscall.Readlink(path, buf)
		if err != nil {
			return "", err
		}
		if n < size {
			return string(buf[:n]), nil
		}
	}
}

// posixSymlink is posix.symlink(src, dst): create dst as a symlink to src. The
// optional target_is_directory flag matters only on Windows, so it is accepted
// and ignored here. os.symlink re-exports it and readlink reads it back.
func posixSymlink(args []objects.Object) (objects.Object, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, objects.Raise(objects.TypeError, "symlink() takes from 2 to 3 arguments (%d given)", len(args))
	}
	src, ok := posixFsPath(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "symlink: src should be string or bytes, not %s", args[0].TypeName())
	}
	dst, ok := posixFsPath(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "symlink: dst should be string or bytes, not %s", args[1].TypeName())
	}
	if err := syscall.Symlink(src, dst); err != nil {
		return nil, posixStatErr(err)
	}
	return objects.None, nil
}

// posixUnlink is posix.unlink(path), re-exported as os.unlink and os.remove: it
// removes a single file. It goes straight to the unlink syscall the way CPython
// does rather than os.Remove, which would also clear an empty directory, so a
// directory raises the host's native errno (EISDIR on Linux, EPERM on macOS)
// exactly as CPython's unlink reports it. A symlink is removed as the link.
// tempfile binds it as a default argument, so it has to exist for the module to
// import.
func posixUnlink(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "unlink() takes exactly 1 argument (%d given)", len(args))
	}
	path, ok := posixFsPath(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "unlink: path should be string or bytes, not %s", args[0].TypeName())
	}
	if err := syscall.Unlink(path); err != nil {
		return nil, posixStatErr(err)
	}
	return objects.None, nil
}

// posixMkdir is posix.mkdir(path, mode=0o777): it creates one directory. The
// mode is masked by the process umask exactly as the C mkdir does, so the bits
// that survive match CPython. It creates only the final component, so a missing
// parent is a FileNotFoundError, not a silent makedirs.
func posixMkdir(args []objects.Object) (objects.Object, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, objects.Raise(objects.TypeError, "mkdir() takes at most 2 arguments (%d given)", len(args))
	}
	path, ok := posixFsPath(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "mkdir: path should be string or bytes, not %s", args[0].TypeName())
	}
	mode := int64(0o777)
	if len(args) == 2 && args[1] != objects.None {
		m, ok := objects.AsInt(args[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[1].TypeName())
		}
		mode = m
	}
	if err := os.Mkdir(path, os.FileMode(mode)); err != nil {
		return nil, posixStatErr(err)
	}
	return objects.None, nil
}

// posixRmdir is posix.rmdir(path): it removes one empty directory. It goes
// straight to the rmdir syscall rather than os.Remove so a non-directory or a
// non-empty directory raises the way CPython does instead of unlinking a file.
func posixRmdir(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "rmdir() takes exactly 1 argument (%d given)", len(args))
	}
	path, ok := posixFsPath(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "rmdir: path should be string or bytes, not %s", args[0].TypeName())
	}
	if err := syscall.Rmdir(path); err != nil {
		return nil, posixStatErr(err)
	}
	return objects.None, nil
}

// posixRename is posix.rename(src, dst): it renames a file or directory,
// replacing dst if it already exists, the same overwrite contract as the C
// rename and CPython on POSIX.
func posixRename(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "rename() takes exactly 2 arguments (%d given)", len(args))
	}
	src, ok := posixFsPath(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "rename: src should be string or bytes, not %s", args[0].TypeName())
	}
	dst, ok := posixFsPath(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "rename: dst should be string or bytes, not %s", args[1].TypeName())
	}
	if err := os.Rename(src, dst); err != nil {
		return nil, posixStatErr(err)
	}
	return objects.None, nil
}

// posixChmod is posix.chmod(path, mode, *, dir_fd=None, follow_symlinks=True),
// re-exported as os.chmod: it sets the permission bits of a str, bytes or
// os.PathLike path. unagi advertises no fd-relative or nofollow capability
// (posix._have_functions is empty, so os.supports_dir_fd and
// os.supports_follow_symlinks are empty), so a non-default dir_fd or
// follow_symlinks=False raises NotImplementedError with CPython's exact text,
// the response a program that honored those capability sets would never trigger.
func posixChmod(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	var pathArg, modeArg, dirFd objects.Object
	followSymlinks := true
	if len(pos) >= 1 {
		pathArg = pos[0]
	}
	if len(pos) >= 2 {
		modeArg = pos[1]
	}
	if len(pos) > 2 {
		return nil, objects.Raise(objects.TypeError, "chmod() takes at most 2 positional arguments (%d given)", len(pos))
	}
	for i, name := range kwNames {
		switch name {
		case "path":
			pathArg = kwVals[i]
		case "mode":
			modeArg = kwVals[i]
		case "dir_fd":
			dirFd = kwVals[i]
		case "follow_symlinks":
			b, err := objects.TruthOf(kwVals[i])
			if err != nil {
				return nil, err
			}
			followSymlinks = b
		default:
			return nil, objects.Raise(objects.TypeError, "chmod() got an unexpected keyword argument '%s'", name)
		}
	}
	if pathArg == nil {
		return nil, objects.Raise(objects.TypeError, "chmod() missing required argument 'path' (pos 1)")
	}
	if modeArg == nil {
		return nil, objects.Raise(objects.TypeError, "chmod() missing required argument 'mode' (pos 2)")
	}
	mode, ok := objects.AsInt(modeArg)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", modeArg.TypeName())
	}
	if dirFd != nil && dirFd != objects.None {
		return nil, objects.Raise("NotImplementedError", "chmod: dir_fd unavailable on this platform")
	}
	if !followSymlinks {
		return nil, objects.Raise("NotImplementedError", "chmod: follow_symlinks unavailable on this platform")
	}
	p, name, ok := posixFsPathName(pathArg)
	if !ok {
		return nil, objects.Raise(objects.TypeError,
			"chmod: path should be string, bytes or os.PathLike, not %s", pathArg.TypeName())
	}
	if err := posixNullCheck("chmod", p); err != nil {
		return nil, err
	}
	if err := syscall.Chmod(p, uint32(mode)); err != nil {
		return nil, posixStatErr(err, name)
	}
	return objects.None, nil
}

// posixFsPath reads a filesystem path argument as a string through the os.fspath
// protocol: a str or bytes path is taken directly, and a path-like object is
// reduced through its __fspath__ (which itself returns str or bytes), the way the
// POSIX calls accept os.PathLike. ok is false for anything else -- an int fd or a
// non-path type -- which each caller reports with its own message.
func posixFsPath(o objects.Object) (string, bool) {
	if b, ok := objects.AsBytes(o); ok {
		return string(b), true
	}
	if s, ok := objects.AsStr(o); ok {
		return s, true
	}
	m, err := objects.LoadAttr(o, "__fspath__")
	if err != nil {
		return "", false
	}
	r, cerr := objects.Call(m, nil)
	if cerr != nil {
		return "", false
	}
	if b, ok := objects.AsBytes(r); ok {
		return string(b), true
	}
	return objects.AsStr(r)
}

// posixFsPathName is posixFsPath plus the filename object an OSError raised on the
// path should carry. CPython's raised error names the fspath-reduced value, not
// the original argument: a str or bytes stays itself, an os.PathLike reduces to
// its __fspath__ result (str or bytes). Reducing once here avoids a second
// __fspath__ call for the error path. The returned object is nil when ok is false.
func posixFsPathName(o objects.Object) (string, objects.Object, bool) {
	if b, ok := objects.AsBytes(o); ok {
		return string(b), o, true
	}
	if s, ok := objects.AsStr(o); ok {
		return s, o, true
	}
	m, err := objects.LoadAttr(o, "__fspath__")
	if err != nil {
		return "", nil, false
	}
	r, cerr := objects.Call(m, nil)
	if cerr != nil {
		return "", nil, false
	}
	if b, ok := objects.AsBytes(r); ok {
		return string(b), r, true
	}
	if s, ok := objects.AsStr(r); ok {
		return s, r, true
	}
	return "", nil, false
}

// posixGetuid is posix.getuid(): the process's real user id. posixpath.expanduser
// reaches for it to resolve a bare ~ with no HOME set.
func posixGetuid(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "getuid() takes no arguments (%d given)", len(args))
	}
	return objects.NewInt(int64(syscall.Getuid())), nil
}

// posixGeteuid is posix.geteuid(): the process's effective user id.
func posixGeteuid(args []objects.Object) (objects.Object, error) {
	if len(args) != 0 {
		return nil, objects.Raise(objects.TypeError, "geteuid() takes no arguments (%d given)", len(args))
	}
	return objects.NewInt(int64(syscall.Geteuid())), nil
}

// posixPutenv is posix.putenv(key, value): it sets an environment variable in
// the process so child processes inherit it. os.py's _Environ.__setitem__ drives
// it with the encoded (surrogateescape bytes) key and value, then updates its own
// bytes dict; str arguments are accepted too. A key holding '=' is rejected the
// way CPython does, since the C environ splits on it.
func posixPutenv(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "putenv() takes exactly 2 arguments (%d given)", len(args))
	}
	key, ok := posixFsPath(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "putenv() argument 1 must be str or bytes, not %s", args[0].TypeName())
	}
	val, ok := posixFsPath(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "putenv() argument 2 must be str or bytes, not %s", args[1].TypeName())
	}
	if strings.ContainsRune(key, '=') {
		return nil, objects.Raise(objects.ValueError, "illegal environment variable name")
	}
	if err := os.Setenv(key, val); err != nil {
		return nil, posixStatErr(err)
	}
	return objects.None, nil
}

// posixUnsetenv is posix.unsetenv(key): it removes an environment variable from
// the process. os.py's _Environ.__delitem__ drives it with the encoded key, then
// deletes its own dict entry.
func posixUnsetenv(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "unsetenv() takes exactly 1 argument (%d given)", len(args))
	}
	key, ok := posixFsPath(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "unsetenv() argument 1 must be str or bytes, not %s", args[0].TypeName())
	}
	if err := os.Unsetenv(key); err != nil {
		return nil, posixStatErr(err)
	}
	return objects.None, nil
}
