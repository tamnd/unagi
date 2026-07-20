package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func TestErrnoNamesAndErrorcode(t *testing.T) {
	mo, err := ImportModule("errno")
	if err != nil {
		t.Fatalf("import errno: %v", err)
	}
	attrInt := func(name string) int64 {
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
	// The low errno numbers are identical on every POSIX host, so they can be
	// asserted directly regardless of the build platform.
	stable := map[string]int64{
		"EPERM": 1, "ENOENT": 2, "EBADF": 9, "EACCES": 13, "EEXIST": 17,
		"EINVAL": 22, "EPIPE": 32, "ERANGE": 34,
	}
	for name, want := range stable {
		if got := attrInt(name); got != want {
			t.Fatalf("errno.%s = %d, want %d", name, got, want)
		}
	}
	// EWOULDBLOCK aliases EAGAIN on every host.
	if attrInt("EWOULDBLOCK") != attrInt("EAGAIN") {
		t.Fatal("EWOULDBLOCK should equal EAGAIN")
	}

	code, err := objects.LoadAttr(mo, "errorcode")
	if err != nil {
		t.Fatalf("errno.errorcode: %v", err)
	}
	// errorcode round-trips each stable number to its name, and keeps the
	// canonical name for a number an alias also maps to.
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
	agn := attrInt("EAGAIN")
	v, err := objects.GetItem(code, objects.NewInt(agn))
	if err != nil {
		t.Fatalf("errorcode[EAGAIN]: %v", err)
	}
	if s, ok := objects.AsStr(v); !ok || s != "EAGAIN" {
		t.Fatalf("errorcode[EAGAIN] = %v, want \"EAGAIN\"", v)
	}
}
