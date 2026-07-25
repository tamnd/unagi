package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestGrpModule checks grp.getgrgid/getgrnam over the host group database. Only
// host-invariant facts are asserted: gid 0 exists on every POSIX host (its name
// is "wheel" on darwin and "root" on Linux, so the name is not asserted, only
// round-tripped), the record is a 4-field struct_group, and a missing name is a
// KeyError with CPython's wording. gr_mem and gr_passwd are host-specific and
// not surfaced by os/user, so they are not asserted.
func TestGrpModule(t *testing.T) {
	mo, err := ImportModule("grp")
	if err != nil {
		t.Fatalf("import grp: %v", err)
	}
	attr := func(name string) objects.Object {
		t.Helper()
		v, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("grp.%s: %v", name, err)
		}
		return v
	}
	call := func(name string, args ...objects.Object) (objects.Object, error) {
		return objects.Call(attr(name), args)
	}
	field := func(v objects.Object, name string) objects.Object {
		t.Helper()
		x, err := objects.LoadAttr(v, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return x
	}

	g, err := call("getgrgid", objects.NewInt(0))
	if err != nil {
		t.Fatalf("getgrgid(0): %v", err)
	}
	if tn := g.TypeName(); tn != "struct_group" {
		t.Errorf("getgrgid type = %s, want struct_group", tn)
	}
	if n, _ := objects.Len(g); n != 4 {
		t.Errorf("len = %d, want 4", n)
	}
	if n, _ := objects.AsInt(field(g, "gr_gid")); n != 0 {
		t.Errorf("gr_gid = %d, want 0", n)
	}

	// The gid-0 group resolves the same by name, proving getgrnam round-trips.
	name, _ := objects.AsStr(field(g, "gr_name"))
	byname, err := call("getgrnam", objects.NewStr(name))
	if err != nil {
		t.Fatalf("getgrnam(%q): %v", name, err)
	}
	if n, _ := objects.AsInt(field(byname, "gr_gid")); n != 0 {
		t.Errorf("getgrnam(%q).gr_gid = %d, want 0", name, n)
	}

	// A name that is not in the database is a KeyError, not a crash.
	if _, err := call("getgrnam", objects.NewStr("zzz_no_such_group")); err == nil {
		t.Errorf("getgrnam(missing) did not raise")
	}
}
