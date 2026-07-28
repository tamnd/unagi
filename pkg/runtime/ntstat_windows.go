//go:build windows

package runtime

import (
	"syscall"

	"github.com/tamnd/unagi/pkg/objects"
)

// The Windows stat family builds os.stat_result the way the posix one does, but
// its structseq has a different shape: Windows has no st_blksize/st_blocks/
// st_rdev and adds st_file_attributes and st_reparse_tag past the birthtime
// fields, matching what CPython's stat_result exposes on Windows. The values
// come from CreateFile + GetFileInformationByHandle rather than a stat(2) call,
// so st_dev/st_ino/st_nlink carry the volume serial, the 64-bit file index and
// the hard-link count the way CPython derives them.
//
// os.py needs the stat name for two module-level references (the _have_functions
// block's `_set.add(stat)` and the fwalk guard `{scandir, stat} <= supports_fd`),
// so registering this is part of what lets `import os` come up on Windows.

// Windows file-attribute bits and the synthetic mode bits CPython folds them
// into. attributes_to_mode in CPython maps a directory to S_IFDIR|0o111, a file
// to S_IFREG, then ORs 0o444 for a read-only entry or 0o666 otherwise, so a
// writable directory reads 0o40777 and a writable file 0o100666, the numbers
// os.stat reports on Windows.
const (
	fileAttrReadonly  = 0x00000001
	fileAttrDirectory = 0x00000010
	fileAttrReparse   = 0x00000400

	sIFDIR = 0x4000
	sIFREG = 0x8000
	sIFLNK = 0xA000

	// fileFlagOpenReparsePoint opens the link itself rather than its target, the
	// lstat path. syscall does not name it, so it is written out.
	fileFlagOpenReparsePoint = 0x00200000
)

// ntStatFields is the Windows stat_result field list in repr order. The first
// ten are the visible sequence (with the three integer time slots unnamed in
// CPython, carried as st_atime/st_mtime/st_ctime here the way the posix table
// does); the rest are named-only.
var ntStatFields = []string{
	"st_mode", "st_ino", "st_dev", "st_nlink", "st_uid", "st_gid", "st_size",
	"st_atime", "st_mtime", "st_ctime",
	"st_atime_ns", "st_mtime_ns", "st_ctime_ns",
	"st_birthtime", "st_birthtime_ns",
	"st_file_attributes", "st_reparse_tag",
}

// ntStatResultType is the Windows stat_result structseq, built once at import.
// n_sequence_fields is 10 and n_unnamed_fields is 3, the same counts CPython
// uses; os.py re-exports the type as os.stat_result.
var ntStatResultType = objects.NewStructSeqType(
	"stat_result", "os.stat_result", ntStatFields, 10, 3,
)

// ntMode folds the Windows file attributes into the synthetic st_mode. follow is
// false on the lstat path, where a reparse point reads as a symlink.
func ntMode(attr uint32, follow bool) int64 {
	var m int64
	if attr&fileAttrDirectory != 0 {
		m |= sIFDIR | 0o111
	} else {
		m |= sIFREG
	}
	if attr&fileAttrReadonly != 0 {
		m |= 0o444
	} else {
		m |= 0o666
	}
	if !follow && attr&fileAttrReparse != 0 {
		m = sIFLNK | (m & 0o777)
	}
	return m
}

// ntStatFromInfo turns a GetFileInformationByHandle result into stat_result.
// uid and gid are always 0 on Windows, the way CPython reports them.
func ntStatFromInfo(bhfi *syscall.ByHandleFileInformation, follow bool) objects.Object {
	mode := ntMode(bhfi.FileAttributes, follow)
	ino := int64(bhfi.FileIndexHigh)<<32 | int64(bhfi.FileIndexLow)
	dev := int64(bhfi.VolumeSerialNumber)
	nlink := int64(bhfi.NumberOfLinks)
	size := int64(bhfi.FileSizeHigh)<<32 | int64(bhfi.FileSizeLow)
	aNs := bhfi.LastAccessTime.Nanoseconds()
	mNs := bhfi.LastWriteTime.Nanoseconds()
	cNs := bhfi.CreationTime.Nanoseconds()
	var reparse int64
	// The reparse tag is not carried by ByHandleFileInformation; a reparse point
	// reports 0 here rather than its specific tag, which the import path and the
	// common queries do not depend on.
	return ntStatBuild(mode, ino, dev, nlink, size, aNs, mNs, cNs, cNs, int64(bhfi.FileAttributes), reparse)
}

// ntStatBuild assembles the structseq. The visible sequence carries the integer
// seconds at slots 7-9 (what old os.stat(p)[7] code reads) while st_atime and
// friends are the float seconds, and st_atime_ns is the exact nanoseconds.
func ntStatBuild(mode, ino, dev, nlink, size, aNs, mNs, cNs, bNs, fileAttrs, reparse int64) objects.Object {
	seq := []objects.Object{
		objects.NewInt(mode), objects.NewInt(ino), objects.NewInt(dev),
		objects.NewInt(nlink), objects.NewInt(0), objects.NewInt(0), objects.NewInt(size),
		objects.NewInt(aNs / 1_000_000_000), objects.NewInt(mNs / 1_000_000_000), objects.NewInt(cNs / 1_000_000_000),
	}
	vals := []objects.Object{
		objects.NewInt(mode), objects.NewInt(ino), objects.NewInt(dev),
		objects.NewInt(nlink), objects.NewInt(0), objects.NewInt(0), objects.NewInt(size),
		objects.NewFloat(float64(aNs) / 1e9), objects.NewFloat(float64(mNs) / 1e9), objects.NewFloat(float64(cNs) / 1e9),
		objects.NewInt(aNs), objects.NewInt(mNs), objects.NewInt(cNs),
		objects.NewFloat(float64(bNs) / 1e9), objects.NewInt(bNs),
		objects.NewInt(fileAttrs), objects.NewInt(reparse),
	}
	return ntStatResultType.NewStructSeq(seq, vals)
}

// ntOpenForStat opens a path for a metadata query with zero access rights, which
// is enough to read the file information and works on directories thanks to the
// backup-semantics flag. follow decides whether a reparse point is traversed.
func ntOpenForStat(path string, follow bool) (syscall.Handle, error) {
	p16, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return syscall.InvalidHandle, err
	}
	flags := uint32(syscall.FILE_FLAG_BACKUP_SEMANTICS)
	if !follow {
		flags |= fileFlagOpenReparsePoint
	}
	return syscall.CreateFile(
		p16, 0,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, flags, 0,
	)
}

// ntStatPath stats a path, following symlinks when follow is true.
func ntStatPath(path string, follow bool) (objects.Object, error) {
	h, err := ntOpenForStat(path, follow)
	if err != nil {
		return nil, ntPathErr(err)
	}
	defer func() { _ = syscall.CloseHandle(h) }()
	var bhfi syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(h, &bhfi); err != nil {
		return nil, ntPathErr(err)
	}
	return ntStatFromInfo(&bhfi, follow), nil
}

func ntStat(args []objects.Object) (objects.Object, error) {
	p, err := ntStatArgPath("stat", args)
	if err != nil {
		return nil, err
	}
	return ntStatPath(p, true)
}

func ntLstat(args []objects.Object) (objects.Object, error) {
	p, err := ntStatArgPath("lstat", args)
	if err != nil {
		return nil, err
	}
	return ntStatPath(p, false)
}

// ntFstat stats an open descriptor. The fd carries a Windows handle widened to
// an int by nt.open, so it is narrowed back to a handle here.
func ntFstat(args []objects.Object) (objects.Object, error) {
	if len(args) != 1 {
		return nil, objects.Raise(objects.TypeError, "fstat() takes exactly 1 argument (%d given)", len(args))
	}
	fd, ok := objects.AsInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required (got type %s)", args[0].TypeName())
	}
	var bhfi syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(fd), &bhfi); err != nil {
		return nil, ntPathErr(err)
	}
	return ntStatFromInfo(&bhfi, true), nil
}

// ntStatArgPath reads the single path argument shared by stat and lstat,
// accepting str and bytes.
func ntStatArgPath(name string, args []objects.Object) (string, error) {
	if len(args) != 1 {
		return "", objects.Raise(objects.TypeError, "%s() takes exactly 1 argument (%d given)", name, len(args))
	}
	p, ok := ntFsPath(args[0])
	if !ok {
		return "", objects.Raise(objects.TypeError, "%s: path should be string, not %s", name, args[0].TypeName())
	}
	return p, nil
}
