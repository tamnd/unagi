//go:build !windows

package runtime

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"github.com/tamnd/unagi/pkg/objects"
)

// The stat family builds os.stat_result, a structseq. Most of the shape is the
// same on every host; a few named fields at the end (st_flags, st_birthtime,
// ...) are platform-specific, so the field names and the syscall.Stat_t
// conversion live in the per-GOOS files posixstat_darwin.go / posixstat_linux.go.
// This file holds the GOOS-agnostic pieces: the common field list, the
// normalized carrier, and the builders that turn a normalized stat into the
// structseq value.

// posixStatCommonFields are the stat_result fields every supported host shares,
// in repr order. The sequence exposes only the first ten (see the int/float
// split in statResult); the rest are named-only. Per-GOOS extras append after.
var posixStatCommonFields = []string{
	"st_mode", "st_ino", "st_dev", "st_nlink", "st_uid", "st_gid", "st_size",
	"st_atime", "st_mtime", "st_ctime",
	"st_atime_ns", "st_mtime_ns", "st_ctime_ns",
	"st_blksize", "st_blocks", "st_rdev",
}

// statNormal is the host-independent view of a stat, filled by the per-GOOS
// statNormalize. Times are kept as (sec, nsec) so this file can derive both the
// float attribute and the int-nanosecond attribute without losing precision.
// extras holds the platform-specific named values, aligned to posixStatExtraNames.
type statNormal struct {
	mode, ino, dev, nlink, uid, gid, size int64
	atimeSec, atimeNsec                   int64
	mtimeSec, mtimeNsec                   int64
	ctimeSec, ctimeNsec                   int64
	blksize, blocks, rdev                 int64
	extras                                []objects.Object
}

// posixStatResultType is the structseq class every stat_result carries. It is
// built once at import; the field list is platform-specific because the extras
// differ per host. n_sequence_fields is 10 and n_unnamed_fields is 3, the same
// on every host CPython supports.
var posixStatResultType = objects.NewStructSeqType(
	"stat_result", "os.stat_result",
	append(append([]string{}, posixStatCommonFields...), posixStatExtraNames...),
	10, 3,
)

// statResult turns a normalized stat into the structseq value. The sequence and
// the named vector diverge at the time fields: the sequence carries the int
// seconds (what old os.stat(p)[stat.ST_MTIME] code expects) while st_atime and
// friends are the float seconds, and st_atime_ns is the exact int nanoseconds.
func statResult(n statNormal) objects.Object {
	seq := []objects.Object{
		objects.NewInt(n.mode), objects.NewInt(n.ino), objects.NewInt(n.dev),
		objects.NewInt(n.nlink), objects.NewInt(n.uid), objects.NewInt(n.gid),
		objects.NewInt(n.size),
		objects.NewInt(n.atimeSec), objects.NewInt(n.mtimeSec), objects.NewInt(n.ctimeSec),
	}
	atimeF := float64(n.atimeSec) + float64(n.atimeNsec)/1e9
	mtimeF := float64(n.mtimeSec) + float64(n.mtimeNsec)/1e9
	ctimeF := float64(n.ctimeSec) + float64(n.ctimeNsec)/1e9
	vals := []objects.Object{
		objects.NewInt(n.mode), objects.NewInt(n.ino), objects.NewInt(n.dev),
		objects.NewInt(n.nlink), objects.NewInt(n.uid), objects.NewInt(n.gid),
		objects.NewInt(n.size),
		objects.NewFloat(atimeF), objects.NewFloat(mtimeF), objects.NewFloat(ctimeF),
		objects.NewInt(n.atimeSec*1_000_000_000 + n.atimeNsec),
		objects.NewInt(n.mtimeSec*1_000_000_000 + n.mtimeNsec),
		objects.NewInt(n.ctimeSec*1_000_000_000 + n.ctimeNsec),
		objects.NewInt(n.blksize), objects.NewInt(n.blocks), objects.NewInt(n.rdev),
	}
	vals = append(vals, n.extras...)
	return posixStatResultType.NewStructSeq(seq, vals)
}

// posixStatErr maps a syscall error to the matching Python exception, structured
// the way CPython raises it: a bare OSError built from the real errno and the
// errno's message, which OSError's own constructor then remaps to the errno's
// subclass (ENOENT -> FileNotFoundError, EACCES -> PermissionError, ...) and
// renders as "[Errno N] Message: 'filename'". The optional filename is the path
// the operation was on — a str, bytes, os.PathLike-reduced value, or an fd
// integer — so a caller that has it (stat, chmod, ...) gets CPython's full
// message; a caller that does not passes none and gets the message without the
// trailing filename, exactly as CPython does for an fd it cannot name.
func posixStatErr(err error, filename ...objects.Object) error {
	var fn objects.Object
	if len(filename) > 0 {
		fn = filename[0]
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		args := []objects.Object{objects.NewInt(int64(errno)), objects.NewStr(posixErrnoMessage(errno))}
		if fn != nil {
			args = append(args, fn)
		}
		// NewException runs the OSError argument split and the errno->subclass
		// remap; the context is chained on the way Raise does for an implicit link.
		e := objects.NewException("OSError", args)
		e.Context = objects.CurrentHandled()
		return e
	}
	// A non-errno error keeps the portable predicates as a fallback; this path is
	// unreachable for the raw syscalls but guards a wrapped error reaching here.
	switch {
	case os.IsNotExist(err):
		return objects.Raise("FileNotFoundError", "%s", err.Error())
	case os.IsPermission(err):
		return objects.Raise("PermissionError", "%s", err.Error())
	}
	return objects.Raise("OSError", "%s", err.Error())
}

// posixErrnoMessage renders an errno's message the way CPython's C strerror does:
// capitalized ("No such file or directory"). Go's syscall.Errno.Error() carries
// the same text but lower-cases the leading word, so the first rune is upper-cased
// to match. The rest of the wording is Go's errno table, close enough to libc for
// the common file errnos the stat family raises.
func posixErrnoMessage(errno syscall.Errno) string {
	s := errno.Error()
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

// posixStatArgN enforces the single positional argument the stat family takes.
func posixStatArgN(name string, args []objects.Object) error {
	if len(args) != 1 {
		return objects.Raise(objects.TypeError, "%s() takes exactly 1 argument (%d given)", name, len(args))
	}
	return nil
}

// posixNullCheck rejects a path carrying an embedded NUL. CPython screens the
// path before the syscall and raises ValueError — not the OSError("invalid
// argument") the kernel would give — with the calling function's name, so a
// caller matching on that text (os.path, test_genericpath) sees exactly it.
func posixNullCheck(name, p string) error {
	if strings.IndexByte(p, 0) >= 0 {
		return objects.Raise(objects.ValueError, "%s: embedded null character in path", name)
	}
	return nil
}

// posixBoolFdWarn fires the RuntimeWarning CPython emits when a bool reaches a
// file-descriptor slot: True and False silently work as the fds 1 and 0, but the
// warning flags the near-certain mistake. It routes through the public warnings
// module so it flows through the same filters and catch_warnings capture as any
// other warning, and is skipped when nothing imported warnings (see
// invertBoolWarn for the same best-effort contract).
func posixBoolFdWarn() error {
	w, err := ImportModule("warnings")
	if err != nil {
		if isModuleNotFound(err) {
			return nil
		}
		return err
	}
	fn, err := objects.LoadAttr(w, "warn")
	if err != nil {
		return err
	}
	cat, ok := objects.ExcClass("RuntimeWarning")
	if !ok {
		return objects.Raise(objects.RuntimeError, "RuntimeWarning class unavailable")
	}
	_, err = objects.CallKwT(objects.MainThread(), fn,
		[]objects.Object{objects.NewStr("bool is used as a file descriptor"), cat},
		[]string{"stacklevel"}, []objects.Object{objects.NewInt(2)})
	return err
}

// posixStat is os.stat: it accepts a str, bytes or os.PathLike path, and also an
// integer file descriptor, which it fstats the way CPython does (a bool counts as
// the fd its int value names).
func posixStat(args []objects.Object) (objects.Object, error) {
	if err := posixStatArgN("stat", args); err != nil {
		return nil, err
	}
	var st syscall.Stat_t
	if p, name, ok := posixFsPathName(args[0]); ok {
		if err := posixNullCheck("stat", p); err != nil {
			return nil, err
		}
		if serr := syscall.Stat(p, &st); serr != nil {
			return nil, posixStatErr(serr, name)
		}
		return statResult(statNormalize(&st)), nil
	}
	if fd, ok := objects.AsInt(args[0]); ok {
		if _, isBool := objects.AsBool(args[0]); isBool {
			if err := posixBoolFdWarn(); err != nil {
				return nil, err
			}
		}
		if serr := syscall.Fstat(int(fd), &st); serr != nil {
			// CPython names the fd itself as the OSError filename for the fd form.
			return nil, posixStatErr(serr, args[0])
		}
		return statResult(statNormalize(&st)), nil
	}
	return nil, objects.Raise(objects.TypeError,
		"stat: path should be string, bytes, os.PathLike or integer, not %s", args[0].TypeName())
}

// posixLstat is os.lstat: like stat over a str, bytes or os.PathLike path, but
// with no file-descriptor form, so a non-path argument is a TypeError.
func posixLstat(args []objects.Object) (objects.Object, error) {
	if err := posixStatArgN("lstat", args); err != nil {
		return nil, err
	}
	p, name, ok := posixFsPathName(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError,
			"lstat: path should be string, bytes or os.PathLike, not %s", args[0].TypeName())
	}
	if err := posixNullCheck("lstat", p); err != nil {
		return nil, err
	}
	var st syscall.Stat_t
	if serr := syscall.Lstat(p, &st); serr != nil {
		return nil, posixStatErr(serr, name)
	}
	return statResult(statNormalize(&st)), nil
}

func posixFstat(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "fstat() takes exactly 1 argument (%d given)", len(args))
	}
	fd, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[0].TypeName())
	}
	var st syscall.Stat_t
	if serr := syscall.Fstat(int(fd), &st); serr != nil {
		// os.fstat leaves the OSError filename None, unlike os.stat's fd form which
		// names the fd; fstat is "stat this already-open fd", with no path to name.
		return nil, posixStatErr(serr)
	}
	return statResult(statNormalize(&st)), nil
}

// posixAccess answers whether the process can access a path with the given mode,
// returning a bool rather than raising: a missing file is False, not an error,
// the same contract as C access and os.access.
func posixAccess(args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "access() takes exactly 2 arguments (%d given)", len(args))
	}
	p, ok := posixFsPath(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError,
			"access: path should be string, bytes or os.PathLike, not %s", args[0].TypeName())
	}
	if err := posixNullCheck("access", p); err != nil {
		return nil, err
	}
	mode, ok := objects.AsInt(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[1].TypeName())
	}
	return objects.NewBool(syscall.Access(p, uint32(mode)) == nil), nil
}
