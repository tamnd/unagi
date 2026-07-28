package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// errnoAttrInt loads errno.<name> as an int, failing the test if it is missing
// or not an int.
func errnoAttrInt(t *testing.T, mo objects.Object, name string) int64 {
	t.Helper()
	v, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("errno.%s: %v", name, err)
	}
	n, ok := objects.AsInt(v)
	if !ok {
		t.Fatalf("errno.%s is not an int", name)
	}
	return n
}

func TestErrnoNamesAndErrorcode(t *testing.T) {
	mo, err := ImportModule("errno")
	if err != nil {
		t.Fatalf("import errno: %v", err)
	}
	// These low errno numbers are identical on every host CPython supports,
	// POSIX and the Windows C runtime alike, so they can be asserted regardless
	// of the build platform.
	stable := map[string]int64{
		"EPERM": 1, "ENOENT": 2, "EBADF": 9, "EACCES": 13, "EEXIST": 17,
		"EINVAL": 22, "EPIPE": 32, "ERANGE": 34,
	}
	for name, want := range stable {
		if got := errnoAttrInt(t, mo, name); got != want {
			t.Fatalf("errno.%s = %d, want %d", name, got, want)
		}
	}

	code, err := objects.LoadAttr(mo, "errorcode")
	if err != nil {
		t.Fatalf("errno.errorcode: %v", err)
	}
	// errorcode round-trips each stable number to its name.
	for name, num := range stable {
		v, err := objects.GetItem(code, objects.NewInt(num))
		if err != nil {
			t.Fatalf("errorcode[%d]: %v", num, err)
		}
		s, ok := objects.AsStr(v)
		if !ok || s != name {
			t.Fatalf("errorcode[%d] = %v, want %q", num, v, name)
		}
	}
}
