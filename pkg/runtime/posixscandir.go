package runtime

import (
	"errors"
	"os"
	"strings"
	"syscall"

	"github.com/tamnd/unagi/pkg/objects"
)

// posix.scandir lists a directory as DirEntry values, the fast path os.walk and
// os.scandir run on. This is sub-slice 6d-2 of the posix arc (Spec 2076 stdlib
// S0_posix_6d_stat_family.md). Two Go classObjects back it: DirEntry, which
// answers name/path plus the is_dir/is_file/is_symlink/stat/inode queries from
// the entry path, and the scandir iterator, a context manager that yields them.
//
// CPython's DirEntry caches the readdir d_type so is_dir does not always stat;
// this keeps the entry path only and re-derives each query with a stat, which is
// a touch slower but the same answer. The iterator reads the whole directory up
// front rather than streaming an open fd, so close is a no-op flag; that is why
// the fixture asserts entry contents, not laziness.

var (
	posixDirEntryClass objects.Object
	posixScandirClass  objects.Object
)

const (
	deNameSlot    = "name"
	dePathSlot    = "path"
	sdEntriesSlot = "_entries"
	sdPosSlot     = "_pos"
	sdClosedSlot  = "_closed"
)

// buildPosixDirEntry constructs the DirEntry class. name and path are plain
// read-only data attributes (stored in slots so the instance __dict__ stays
// empty the way the C DirEntry's does); everything else is a query method.
func buildPosixDirEntry() (objects.Object, error) {
	slots := objects.NewTuple([]objects.Object{objects.NewStr(deNameSlot), objects.NewStr(dePathSlot)})
	names := []string{
		"__slots__", "__init__",
		"__fspath__",
		"inode", "is_dir", "is_file", "is_symlink", "is_junction",
		"stat", "__repr__",
	}
	vals := []objects.Object{
		slots,
		objects.NewMethod("__init__", 3, direntryInit),
		objects.NewMethod("__fspath__", 1, func(args []objects.Object) (objects.Object, error) {
			return objects.LoadAttr(args[0], dePathSlot)
		}),
		objects.NewMethod("inode", 1, direntryInode),
		objects.NewMethodKw("is_dir", direntryIsDir),
		objects.NewMethodKw("is_file", direntryIsFile),
		objects.NewMethod("is_symlink", 1, direntryIsSymlink),
		objects.NewMethod("is_junction", 1, func([]objects.Object) (objects.Object, error) {
			// Junctions are a Windows concept; a POSIX DirEntry is never one.
			return objects.False, nil
		}),
		objects.NewMethodKw("stat", direntryStat),
		objects.NewMethod("__repr__", 1, direntryRepr),
	}
	return objects.NewClass("DirEntry", "posix.DirEntry", nil, names, vals, nil, nil)
}

func direntryInit(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := objects.StoreAttr(self, deNameSlot, args[1]); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, dePathSlot, args[2]); err != nil {
		return nil, err
	}
	return objects.None, nil
}

func direntryPathStr(self objects.Object) (string, error) {
	p, err := objects.LoadAttr(self, dePathSlot)
	if err != nil {
		return "", err
	}
	s, ok := objects.AsStr(p)
	if !ok {
		return "", objects.Raise(objects.TypeError, "DirEntry.path is not a str")
	}
	return s, nil
}

// direntryMode stats the entry path and returns the raw mode plus whether the
// path still exists. A vanished entry reports exists=false rather than raising,
// so is_dir/is_file can swallow it the way CPython's DirEntry does.
func direntryMode(self objects.Object, follow bool) (int64, bool, error) {
	p, err := direntryPathStr(self)
	if err != nil {
		return 0, false, err
	}
	var st syscall.Stat_t
	var serr error
	if follow {
		serr = syscall.Stat(p, &st)
	} else {
		serr = syscall.Lstat(p, &st)
	}
	if serr != nil {
		if os.IsNotExist(serr) {
			return 0, false, nil
		}
		return 0, false, posixStatErr(serr)
	}
	return int64(st.Mode), true, nil
}

// followSymlinks reads the follow_symlinks keyword, which defaults to True on
// is_dir/is_file/stat. The methods are keyword-only in CPython, so only a
// keyword argument is honoured.
func followSymlinks(kwNames []string, kwVals []objects.Object) bool {
	for i, n := range kwNames {
		if n == "follow_symlinks" {
			return objects.Truth(kwVals[i])
		}
	}
	return true
}

func direntryTypeQuery(pos []objects.Object, kwNames []string, kwVals []objects.Object, typeBits int64) (objects.Object, error) {
	mode, exists, err := direntryMode(pos[0], followSymlinks(kwNames, kwVals))
	if err != nil {
		return nil, err
	}
	if !exists {
		return objects.False, nil
	}
	return objects.NewBool(mode&syscall.S_IFMT == typeBits), nil
}

func direntryIsDir(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	return direntryTypeQuery(pos, kwNames, kwVals, syscall.S_IFDIR)
}

func direntryIsFile(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	return direntryTypeQuery(pos, kwNames, kwVals, syscall.S_IFREG)
}

func direntryIsSymlink(args []objects.Object) (objects.Object, error) {
	mode, exists, err := direntryMode(args[0], false)
	if err != nil {
		return nil, err
	}
	if !exists {
		return objects.False, nil
	}
	return objects.NewBool(mode&syscall.S_IFMT == syscall.S_IFLNK), nil
}

// direntryInode returns the entry's inode number. CPython caches d_ino from
// readdir; this re-reads it with an lstat, which matches for a live entry.
func direntryInode(args []objects.Object) (objects.Object, error) {
	p, err := direntryPathStr(args[0])
	if err != nil {
		return nil, err
	}
	var st syscall.Stat_t
	if serr := syscall.Lstat(p, &st); serr != nil {
		return nil, posixStatErr(serr)
	}
	return objects.NewInt(int64(st.Ino)), nil
}

// direntryStat returns a full stat_result for the entry, following symlinks by
// default. Unlike is_dir it propagates a stat error rather than swallowing it.
func direntryStat(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	p, err := direntryPathStr(pos[0])
	if err != nil {
		return nil, err
	}
	arg := []objects.Object{objects.NewStr(p)}
	if followSymlinks(kwNames, kwVals) {
		return posixStat(arg)
	}
	return posixLstat(arg)
}

func direntryRepr(args []objects.Object) (objects.Object, error) {
	name, err := objects.LoadAttr(args[0], deNameSlot)
	if err != nil {
		return nil, err
	}
	r, err := objects.ReprE(name)
	if err != nil {
		return nil, err
	}
	return objects.NewStr("<DirEntry " + r + ">"), nil
}

// buildPosixScandir constructs the scandir iterator class. The entries are read
// eagerly into a slot list; the iterator walks it with a cursor and treats close
// as setting a flag that makes __next__ stop.
func buildPosixScandir() (objects.Object, error) {
	slots := objects.NewTuple([]objects.Object{
		objects.NewStr(sdEntriesSlot), objects.NewStr(sdPosSlot), objects.NewStr(sdClosedSlot),
	})
	names := []string{
		"__slots__", "__init__",
		"__iter__", "__next__", "__enter__", "__exit__", "close",
	}
	vals := []objects.Object{
		slots,
		objects.NewMethod("__init__", 2, scandirInit),
		objects.NewMethod("__iter__", 1, func(args []objects.Object) (objects.Object, error) {
			return args[0], nil
		}),
		objects.NewMethod("__next__", 1, scandirNext),
		objects.NewMethod("__enter__", 1, func(args []objects.Object) (objects.Object, error) {
			return args[0], nil
		}),
		objects.NewMethod("__exit__", -1, scandirClose),
		objects.NewMethod("close", 1, scandirClose),
	}
	return objects.NewClass("ScandirIterator", "posix.ScandirIterator", nil, names, vals, nil, nil)
}

func scandirInit(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := objects.StoreAttr(self, sdEntriesSlot, args[1]); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, sdPosSlot, objects.NewInt(0)); err != nil {
		return nil, err
	}
	return objects.None, objects.StoreAttr(self, sdClosedSlot, objects.False)
}

func scandirNext(args []objects.Object) (objects.Object, error) {
	self := args[0]
	closed, err := objects.LoadAttr(self, sdClosedSlot)
	if err != nil {
		return nil, err
	}
	entries, err := objects.LoadAttr(self, sdEntriesSlot)
	if err != nil {
		return nil, err
	}
	posObj, err := objects.LoadAttr(self, sdPosSlot)
	if err != nil {
		return nil, err
	}
	pos, _ := objects.AsInt(posObj)
	n, err := objects.Len(entries)
	if err != nil {
		return nil, err
	}
	if objects.Truth(closed) || int(pos) >= n {
		return nil, objects.NewException("StopIteration", nil)
	}
	e, err := objects.GetItem(entries, objects.NewInt(pos))
	if err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, sdPosSlot, objects.NewInt(pos+1)); err != nil {
		return nil, err
	}
	return e, nil
}

func scandirClose(args []objects.Object) (objects.Object, error) {
	return objects.None, objects.StoreAttr(args[0], sdClosedSlot, objects.True)
}

// scandirJoin builds a DirEntry path the way CPython does: the scandir argument
// followed by the entry name, with a separator inserted only when the argument
// does not already end in one. An empty argument yields the bare name.
func scandirJoin(dir, name string) string {
	switch {
	case dir == "":
		return name
	case strings.HasSuffix(dir, "/"):
		return dir + name
	default:
		return dir + "/" + name
	}
}

// posixScandir implements posix.scandir. It accepts a str path, defaulting to
// the current directory; bytes paths and directory file descriptors are not
// supported yet.
func posixScandir(args []objects.Object) (objects.Object, error) {
	if len(args) > 1 {
		return nil, objects.Raise(objects.TypeError, "scandir() takes at most 1 argument (%d given)", len(args))
	}
	dir := "."
	if len(args) == 1 && args[0] != objects.None {
		s, ok := objects.AsStr(args[0])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "scandir: path should be string, not %s", args[0].TypeName())
		}
		dir = s
	}
	names, err := scandirNames(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]objects.Object, len(names))
	for i, name := range names {
		e, err := objects.Call(posixDirEntryClass, []objects.Object{
			objects.NewStr(name), objects.NewStr(scandirJoin(dir, name)),
		})
		if err != nil {
			return nil, err
		}
		entries[i] = e
	}
	return objects.Call(posixScandirClass, []objects.Object{objects.NewList(entries)})
}

// scandirNames reads a directory's entry names, mapping the syscall errors to
// the exceptions CPython raises: a missing path is FileNotFoundError and a
// non-directory is NotADirectoryError.
func scandirNames(dir string) ([]string, error) {
	f, err := os.Open(dir)
	if err != nil {
		if errors.Is(err, syscall.ENOTDIR) {
			return nil, objects.Raise("NotADirectoryError", "%s", err.Error())
		}
		return nil, posixStatErr(err)
	}
	defer func() { _ = f.Close() }()
	names, err := f.Readdirnames(-1)
	if err != nil {
		if errors.Is(err, syscall.ENOTDIR) {
			return nil, objects.Raise("NotADirectoryError", "%s", err.Error())
		}
		return nil, objects.Raise("OSError", "%s", err.Error())
	}
	return names, nil
}
