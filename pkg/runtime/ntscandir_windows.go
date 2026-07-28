//go:build windows

package runtime

import (
	"errors"
	"os"
	"strings"
	"syscall"

	"github.com/tamnd/unagi/pkg/objects"
)

// nt.scandir lists a directory as DirEntry values, the fast path os.walk and
// os.scandir run on, the Windows analog of posix.scandir. Two Go classObjects
// back it: DirEntry, which answers name/path plus is_dir/is_file/is_symlink/
// is_junction/stat/inode from the entry path, and the scandir iterator, a
// context manager that yields them. The entries are read eagerly into a slot
// list, so close is a flag rather than an open handle, matching the posix build.
//
// os.py re-exports DirEntry and drives scandir from os.walk, and the fwalk guard
// at module scope references the scandir name, so registering this is part of
// letting `import os` come up on Windows.

var (
	ntDirEntryClass objects.Object
	ntScandirClass  objects.Object
)

const (
	ntDeNameSlot    = "name"
	ntDePathSlot    = "path"
	ntSdEntriesSlot = "_entries"
	ntSdPosSlot     = "_pos"
	ntSdClosedSlot  = "_closed"
)

func buildNtDirEntry() (objects.Object, error) {
	slots := objects.NewTuple([]objects.Object{objects.NewStr(ntDeNameSlot), objects.NewStr(ntDePathSlot)})
	names := []string{
		"__slots__", "__init__", "__fspath__",
		"inode", "is_dir", "is_file", "is_symlink", "is_junction",
		"stat", "__repr__",
	}
	vals := []objects.Object{
		slots,
		objects.NewMethod("__init__", 3, ntDirentryInit),
		objects.NewMethod("__fspath__", 1, func(args []objects.Object) (objects.Object, error) {
			return objects.LoadAttr(args[0], ntDePathSlot)
		}),
		objects.NewMethod("inode", 1, ntDirentryInode),
		objects.NewMethodKw("is_dir", ntDirentryIsDir),
		objects.NewMethodKw("is_file", ntDirentryIsFile),
		objects.NewMethod("is_symlink", 1, ntDirentryIsSymlink),
		objects.NewMethod("is_junction", 1, ntDirentryIsJunction),
		objects.NewMethodKw("stat", ntDirentryStat),
		objects.NewMethod("__repr__", 1, ntDirentryRepr),
	}
	return objects.NewClass("DirEntry", "nt.DirEntry", nil, names, vals, nil, nil)
}

func ntDirentryInit(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := objects.StoreAttr(self, ntDeNameSlot, args[1]); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, ntDePathSlot, args[2]); err != nil {
		return nil, err
	}
	return objects.None, nil
}

func ntDirentryPathStr(self objects.Object) (string, error) {
	p, err := objects.LoadAttr(self, ntDePathSlot)
	if err != nil {
		return "", err
	}
	s, ok := objects.AsStr(p)
	if !ok {
		return "", objects.Raise(objects.TypeError, "DirEntry.path is not a str")
	}
	return s, nil
}

// ntDirentryInfo stats the entry path, following symlinks when follow is set. A
// vanished entry reports exists=false rather than raising, so is_dir/is_file can
// swallow it the way CPython's DirEntry does.
func ntDirentryInfo(self objects.Object, follow bool) (os.FileInfo, bool, error) {
	p, err := ntDirentryPathStr(self)
	if err != nil {
		return nil, false, err
	}
	var info os.FileInfo
	var serr error
	if follow {
		info, serr = os.Stat(p)
	} else {
		info, serr = os.Lstat(p)
	}
	if serr != nil {
		if os.IsNotExist(serr) {
			return nil, false, nil
		}
		return nil, false, ntPathErr(serr)
	}
	return info, true, nil
}

func ntFollowSymlinks(kwNames []string, kwVals []objects.Object) bool {
	for i, n := range kwNames {
		if n == "follow_symlinks" {
			return objects.Truth(kwVals[i])
		}
	}
	return true
}

func ntDirentryIsDir(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	info, exists, err := ntDirentryInfo(pos[0], ntFollowSymlinks(kwNames, kwVals))
	if err != nil {
		return nil, err
	}
	if !exists {
		return objects.False, nil
	}
	return objects.NewBool(info.IsDir()), nil
}

func ntDirentryIsFile(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	info, exists, err := ntDirentryInfo(pos[0], ntFollowSymlinks(kwNames, kwVals))
	if err != nil {
		return nil, err
	}
	if !exists {
		return objects.False, nil
	}
	return objects.NewBool(info.Mode().IsRegular()), nil
}

func ntDirentryIsSymlink(args []objects.Object) (objects.Object, error) {
	info, exists, err := ntDirentryInfo(args[0], false)
	if err != nil {
		return nil, err
	}
	if !exists {
		return objects.False, nil
	}
	return objects.NewBool(info.Mode()&os.ModeSymlink != 0), nil
}

// ntDirentryIsJunction reports whether the entry is a Windows junction. Go marks
// a junction with ModeIrregular alongside the reparse attribute, which a plain
// symlink does not carry, so the two are told apart.
func ntDirentryIsJunction(args []objects.Object) (objects.Object, error) {
	info, exists, err := ntDirentryInfo(args[0], false)
	if err != nil {
		return nil, err
	}
	if !exists {
		return objects.False, nil
	}
	m := info.Mode()
	return objects.NewBool(m&os.ModeIrregular != 0 && m&os.ModeSymlink == 0), nil
}

func ntDirentryInode(args []objects.Object) (objects.Object, error) {
	p, err := ntDirentryPathStr(args[0])
	if err != nil {
		return nil, err
	}
	h, serr := ntOpenForStat(p, false)
	if serr != nil {
		return nil, ntPathErr(serr)
	}
	defer func() { _ = syscall.CloseHandle(h) }()
	var bhfi syscall.ByHandleFileInformation
	if serr := syscall.GetFileInformationByHandle(h, &bhfi); serr != nil {
		return nil, ntPathErr(serr)
	}
	return objects.NewInt(int64(bhfi.FileIndexHigh)<<32 | int64(bhfi.FileIndexLow)), nil
}

func ntDirentryStat(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	p, err := ntDirentryPathStr(pos[0])
	if err != nil {
		return nil, err
	}
	return ntStatPath(p, ntFollowSymlinks(kwNames, kwVals))
}

func ntDirentryRepr(args []objects.Object) (objects.Object, error) {
	name, err := objects.LoadAttr(args[0], ntDeNameSlot)
	if err != nil {
		return nil, err
	}
	r, err := objects.ReprE(name)
	if err != nil {
		return nil, err
	}
	return objects.NewStr("<DirEntry " + r + ">"), nil
}

func buildNtScandir() (objects.Object, error) {
	slots := objects.NewTuple([]objects.Object{
		objects.NewStr(ntSdEntriesSlot), objects.NewStr(ntSdPosSlot), objects.NewStr(ntSdClosedSlot),
	})
	names := []string{
		"__slots__", "__init__",
		"__iter__", "__next__", "__enter__", "__exit__", "close",
	}
	vals := []objects.Object{
		slots,
		objects.NewMethod("__init__", 2, ntScandirInit),
		objects.NewMethod("__iter__", 1, func(args []objects.Object) (objects.Object, error) {
			return args[0], nil
		}),
		objects.NewMethod("__next__", 1, ntScandirNext),
		objects.NewMethod("__enter__", 1, func(args []objects.Object) (objects.Object, error) {
			return args[0], nil
		}),
		objects.NewMethod("__exit__", -1, ntScandirClose),
		objects.NewMethod("close", 1, ntScandirClose),
	}
	return objects.NewClass("ScandirIterator", "nt.ScandirIterator", nil, names, vals, nil, nil)
}

func ntScandirInit(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := objects.StoreAttr(self, ntSdEntriesSlot, args[1]); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, ntSdPosSlot, objects.NewInt(0)); err != nil {
		return nil, err
	}
	return objects.None, objects.StoreAttr(self, ntSdClosedSlot, objects.False)
}

func ntScandirNext(args []objects.Object) (objects.Object, error) {
	self := args[0]
	closed, err := objects.LoadAttr(self, ntSdClosedSlot)
	if err != nil {
		return nil, err
	}
	entries, err := objects.LoadAttr(self, ntSdEntriesSlot)
	if err != nil {
		return nil, err
	}
	posObj, err := objects.LoadAttr(self, ntSdPosSlot)
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
	if err := objects.StoreAttr(self, ntSdPosSlot, objects.NewInt(pos+1)); err != nil {
		return nil, err
	}
	return e, nil
}

func ntScandirClose(args []objects.Object) (objects.Object, error) {
	return objects.None, objects.StoreAttr(args[0], ntSdClosedSlot, objects.True)
}

// ntScandirJoin builds a DirEntry path: the scandir argument followed by the
// entry name, with a backslash inserted only when the argument does not already
// end in a separator. An empty argument yields the bare name.
func ntScandirJoin(dir, name string) string {
	switch {
	case dir == "":
		return name
	case strings.HasSuffix(dir, "\\") || strings.HasSuffix(dir, "/"):
		return dir + name
	default:
		return dir + "\\" + name
	}
}

func ntScandir(args []objects.Object) (objects.Object, error) {
	if len(args) > 1 {
		return nil, objects.Raise(objects.TypeError, "scandir() takes at most 1 argument (%d given)", len(args))
	}
	dir := "."
	if len(args) == 1 && args[0] != objects.None {
		s, ok := ntFsPath(args[0])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "scandir: path should be string, not %s", args[0].TypeName())
		}
		dir = s
	}
	names, err := ntScandirNames(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]objects.Object, len(names))
	for i, name := range names {
		e, err := objects.Call(ntDirEntryClass, []objects.Object{
			objects.NewStr(name), objects.NewStr(ntScandirJoin(dir, name)),
		})
		if err != nil {
			return nil, err
		}
		entries[i] = e
	}
	return objects.Call(ntScandirClass, []objects.Object{objects.NewList(entries)})
}

// ntScandirNames reads a directory's entry names, mapping a missing path to
// FileNotFoundError and a non-directory to NotADirectoryError the way CPython
// does.
func ntScandirNames(dir string) ([]string, error) {
	f, err := os.Open(dir)
	if err != nil {
		if errors.Is(err, syscall.ENOTDIR) {
			return nil, objects.Raise("NotADirectoryError", "%s", err.Error())
		}
		return nil, ntPathErr(err)
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
