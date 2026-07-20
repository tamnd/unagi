package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func TestStatConstantsAndHelpers(t *testing.T) {
	mo, err := ImportModule("_stat")
	if err != nil {
		t.Fatalf("import _stat: %v", err)
	}
	attr := func(name string) objects.Object {
		t.Helper()
		v, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("_stat.%s: %v", name, err)
		}
		return v
	}
	attrInt := func(name string) int64 {
		t.Helper()
		n, ok := objects.AsInt(attr(name))
		if !ok {
			t.Fatalf("_stat.%s is not an int", name)
		}
		return n
	}
	// The mode bits are POSIX-universal, so they are asserted by value.
	consts := map[string]int64{
		"S_IFDIR": 0o040000, "S_IFCHR": 0o020000, "S_IFBLK": 0o060000,
		"S_IFREG": 0o100000, "S_IFIFO": 0o010000, "S_IFLNK": 0o120000,
		"S_IFSOCK": 0o140000, "S_IFDOOR": 0, "S_ISUID": 0o4000,
		"S_ISGID": 0o2000, "S_ENFMT": 0o2000, "S_ISVTX": 0o1000,
		"S_IRWXU": 0o700, "S_IRWXG": 0o070, "S_IRWXO": 0o007,
		"ST_MODE": 0, "ST_SIZE": 6, "ST_CTIME": 9,
	}
	for name, want := range consts {
		if got := attrInt(name); got != want {
			t.Errorf("_stat.%s = %#o, want %#o", name, got, want)
		}
	}

	call := func(name string, arg objects.Object) objects.Object {
		t.Helper()
		v, err := objects.Call(attr(name), []objects.Object{arg})
		if err != nil {
			t.Fatalf("_stat.%s: %v", name, err)
		}
		return v
	}
	wantInt := func(name string, arg, want int64) {
		t.Helper()
		if n, ok := objects.AsInt(call(name, objects.NewInt(arg))); !ok || n != want {
			t.Errorf("_stat.%s(%#o) = %v, want %#o", name, arg, call(name, objects.NewInt(arg)), want)
		}
	}
	wantInt("S_IMODE", 0o100644, 0o644)
	wantInt("S_IFMT", 0o100644, 0o100000)

	wantBool := func(name string, arg int64, want bool) {
		t.Helper()
		if got := objects.Truth(call(name, objects.NewInt(arg))); got != want {
			t.Errorf("_stat.%s(%#o) = %v, want %v", name, arg, got, want)
		}
	}
	wantBool("S_ISDIR", 0o040755, true)
	wantBool("S_ISDIR", 0o100644, false)
	wantBool("S_ISREG", 0o100644, true)
	wantBool("S_ISLNK", 0o120777, true)
	wantBool("S_ISCHR", 0o020600, true)
	wantBool("S_ISBLK", 0o060600, true)
	wantBool("S_ISFIFO", 0o010644, true)
	wantBool("S_ISSOCK", 0o140755, true)
	wantBool("S_ISDOOR", 0o100644, false)
	wantBool("S_ISPORT", 0o100644, false)
	wantBool("S_ISWHT", 0o100644, false)

	modes := map[int64]string{
		0o100644: "-rw-r--r--",
		0o040755: "drwxr-xr-x",
		0o120777: "lrwxrwxrwx",
		0o104755: "-rwsr-xr-x",
		0o102644: "-rw-r-Sr--",
		0o041777: "drwxrwxrwt",
		0o000644: "?rw-r--r--",
	}
	for mode, want := range modes {
		v := call("filemode", objects.NewInt(mode))
		if s, ok := objects.AsStr(v); !ok || s != want {
			t.Errorf("filemode(%#o) = %v, want %q", mode, v, want)
		}
	}

	// A non-integer mode is a TypeError, not a panic.
	if _, err := objects.Call(attr("S_IMODE"), []objects.Object{objects.NewStr("x")}); err == nil {
		t.Error("S_IMODE(str) should raise")
	}
}
