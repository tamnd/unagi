package runtime

import (
	"os"
	"syscall"

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

// posixStatErr maps a syscall error to the matching Python exception. stat and
// friends raise FileNotFoundError / PermissionError as the special cases callers
// catch, and plain OSError otherwise.
func posixStatErr(err error) error {
	switch {
	case os.IsNotExist(err):
		return objects.Raise("FileNotFoundError", "%s", err.Error())
	case os.IsPermission(err):
		return objects.Raise("PermissionError", "%s", err.Error())
	}
	return objects.Raise("OSError", "%s", err.Error())
}

// posixStatArgPath reads the single string path argument shared by stat/lstat.
func posixStatArgPath(name string, args []objects.Object) (string, error) {
	if len(args) != 1 {
		return "", objects.Raise(objects.TypeError, "%s() takes exactly 1 argument (%d given)", name, len(args))
	}
	p, ok := objects.AsStr(args[0])
	if !ok {
		return "", objects.Raise(objects.TypeError, "%s: path should be string, not %s", name, args[0].TypeName())
	}
	return p, nil
}

func posixStat(args []objects.Object) (objects.Object, error) {
	p, err := posixStatArgPath("stat", args)
	if err != nil {
		return nil, err
	}
	var st syscall.Stat_t
	if serr := syscall.Stat(p, &st); serr != nil {
		return nil, posixStatErr(serr)
	}
	return statResult(statNormalize(&st)), nil
}

func posixLstat(args []objects.Object) (objects.Object, error) {
	p, err := posixStatArgPath("lstat", args)
	if err != nil {
		return nil, err
	}
	var st syscall.Stat_t
	if serr := syscall.Lstat(p, &st); serr != nil {
		return nil, posixStatErr(serr)
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
	p, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "access: path should be string, not %s", args[0].TypeName())
	}
	mode, ok := objects.AsInt(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[1].TypeName())
	}
	return objects.NewBool(syscall.Access(p, uint32(mode)) == nil), nil
}
