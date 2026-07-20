package runtime

import (
	"strings"

	"github.com/tamnd/unagi/pkg/objects"
)

// _stat is the C accelerator behind stat.py. It provides the file-mode
// constants and the S_IS*/S_IMODE/S_IFMT/filemode helpers that classify a
// stat_result's st_mode. stat.py defines all of this in pure Python and then
// does `from _stat import *` to pick up the C version when present, so this
// module is the override that CPython's stat module runs on.
//
// Unlike errno, the numbers here are POSIX-universal: the S_IF* type bits and
// the S_I* permission bits have the same octal values on darwin and Linux, so
// they are written out directly rather than sourced from syscall. The
// platform-divergent flags a real darwin/BSD _stat also exports (SF_*, UF_*)
// are deliberately left out; stat.py defines its own portable set of those, and
// omitting them here keeps that pure definition in place across platforms.

func init() {
	moduleTable["_stat"] = &moduleEntry{builtin: true, exec: initStat}
}

// statConsts is the portable name->value surface: the stat_result index
// positions, the file-type bits used by S_IFMT, and the permission bits.
var statConsts = []struct {
	name string
	num  int64
}{
	// stat_result tuple indices.
	{"ST_MODE", 0},
	{"ST_INO", 1},
	{"ST_DEV", 2},
	{"ST_NLINK", 3},
	{"ST_UID", 4},
	{"ST_GID", 5},
	{"ST_SIZE", 6},
	{"ST_ATIME", 7},
	{"ST_MTIME", 8},
	{"ST_CTIME", 9},
	// File type bits, the values S_IFMT masks out.
	{"S_IFDIR", 0o040000},
	{"S_IFCHR", 0o020000},
	{"S_IFBLK", 0o060000},
	{"S_IFREG", 0o100000},
	{"S_IFIFO", 0o010000},
	{"S_IFLNK", 0o120000},
	{"S_IFSOCK", 0o140000},
	// Types without a portable value; CPython reports 0 off these platforms.
	{"S_IFDOOR", 0},
	{"S_IFPORT", 0},
	{"S_IFWHT", 0},
	// Permission and special-mode bits.
	{"S_ISUID", 0o4000},
	{"S_ISGID", 0o2000},
	{"S_ENFMT", 0o2000},
	{"S_ISVTX", 0o1000},
	{"S_IREAD", 0o0400},
	{"S_IWRITE", 0o0200},
	{"S_IEXEC", 0o0100},
	{"S_IRWXU", 0o0700},
	{"S_IRUSR", 0o0400},
	{"S_IWUSR", 0o0200},
	{"S_IXUSR", 0o0100},
	{"S_IRWXG", 0o0070},
	{"S_IRGRP", 0o0040},
	{"S_IWGRP", 0o0020},
	{"S_IXGRP", 0o0010},
	{"S_IRWXO", 0o0007},
	{"S_IROTH", 0o0004},
	{"S_IWOTH", 0o0002},
	{"S_IXOTH", 0o0001},
}

const (
	statSIFMT  = 0o170000
	statSIMODE = 0o7777
)

// The file-type bits, kept as Go constants so the helpers do not re-read them
// through module attributes.
const (
	statSIFDIR  = 0o040000
	statSIFCHR  = 0o020000
	statSIFBLK  = 0o060000
	statSIFREG  = 0o100000
	statSIFIFO  = 0o010000
	statSIFLNK  = 0o120000
	statSIFSOCK = 0o140000
)

func initStat(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	for _, c := range statConsts {
		if err := set(c.name, objects.NewInt(c.num)); err != nil {
			return err
		}
	}
	fns := []struct {
		name string
		fn   func([]objects.Object) (objects.Object, error)
	}{
		{"S_IMODE", statSIMODEFn},
		{"S_IFMT", statSIFMTFn},
		{"S_ISDIR", statIsType(statSIFDIR)},
		{"S_ISCHR", statIsType(statSIFCHR)},
		{"S_ISBLK", statIsType(statSIFBLK)},
		{"S_ISREG", statIsType(statSIFREG)},
		{"S_ISFIFO", statIsType(statSIFIFO)},
		{"S_ISLNK", statIsType(statSIFLNK)},
		{"S_ISSOCK", statIsType(statSIFSOCK)},
		{"S_ISDOOR", statIsFalse},
		{"S_ISPORT", statIsFalse},
		{"S_ISWHT", statIsFalse},
		{"filemode", statFilemode},
	}
	for _, f := range fns {
		if err := set(f.name, objects.NewFunc(f.name, 1, f.fn)); err != nil {
			return err
		}
	}
	return nil
}

// statMode reads the single integer mode argument the helpers take.
func statMode(name string, args []objects.Object) (int64, error) {
	if len(args) != 1 {
		return 0, objects.Raise(objects.TypeError, "%s() takes exactly one argument (%d given)", name, len(args))
	}
	mode, ok := objects.AsInt(args[0])
	if !ok {
		return 0, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
	}
	return mode, nil
}

func statSIMODEFn(args []objects.Object) (objects.Object, error) {
	mode, err := statMode("S_IMODE", args)
	if err != nil {
		return nil, err
	}
	return objects.NewInt(mode & statSIMODE), nil
}

func statSIFMTFn(args []objects.Object) (objects.Object, error) {
	mode, err := statMode("S_IFMT", args)
	if err != nil {
		return nil, err
	}
	return objects.NewInt(mode & statSIFMT), nil
}

// statIsType builds an S_ISxxx predicate testing S_IFMT(mode) against one type.
func statIsType(typ int64) func([]objects.Object) (objects.Object, error) {
	return func(args []objects.Object) (objects.Object, error) {
		mode, err := statMode("S_IS", args)
		if err != nil {
			return nil, err
		}
		return objects.NewBool(mode&statSIFMT == typ), nil
	}
}

// statIsFalse backs S_ISDOOR/S_ISPORT/S_ISWHT, which are always False off the
// platforms that define those types; it still type-checks its argument.
func statIsFalse(args []objects.Object) (objects.Object, error) {
	if _, err := statMode("S_IS", args); err != nil {
		return nil, err
	}
	return objects.NewBool(false), nil
}

// filemodeGroup is one column group of the mode string: the first group picks a
// single type char, the rest test permission bits in order and fall back to the
// group's "-" when none match.
type filemodeGroup struct {
	bits  []int64
	chars []string
}

// statFilemodeTable mirrors stat.py's _filemode_table: the type column then the
// nine permission columns, with the setuid/setgid/sticky combinations ahead of
// the plain execute bits so the "s"/"t" forms win.
var statFilemodeTable = []filemodeGroup{
	{
		bits:  []int64{statSIFLNK, statSIFSOCK, statSIFREG, statSIFBLK, statSIFDIR, statSIFCHR, statSIFIFO},
		chars: []string{"l", "s", "-", "b", "d", "c", "p"},
	},
	{bits: []int64{0o0400}, chars: []string{"r"}},
	{bits: []int64{0o0200}, chars: []string{"w"}},
	{bits: []int64{0o0100 | 0o4000, 0o4000, 0o0100}, chars: []string{"s", "S", "x"}},
	{bits: []int64{0o0040}, chars: []string{"r"}},
	{bits: []int64{0o0020}, chars: []string{"w"}},
	{bits: []int64{0o0010 | 0o2000, 0o2000, 0o0010}, chars: []string{"s", "S", "x"}},
	{bits: []int64{0o0004}, chars: []string{"r"}},
	{bits: []int64{0o0002}, chars: []string{"w"}},
	{bits: []int64{0o0001 | 0o1000, 0o1000, 0o0001}, chars: []string{"t", "T", "x"}},
}

func statFilemode(args []objects.Object) (objects.Object, error) {
	mode, err := statMode("filemode", args)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	for i, g := range statFilemodeTable {
		matched := false
		for j, bit := range g.bits {
			if i == 0 {
				if mode&statSIFMT == bit {
					b.WriteString(g.chars[j])
					matched = true
					break
				}
			} else if mode&bit == bit {
				b.WriteString(g.chars[j])
				matched = true
				break
			}
		}
		if !matched {
			if i == 0 {
				b.WriteString("?")
			} else {
				b.WriteString("-")
			}
		}
	}
	return objects.NewStr(b.String()), nil
}
