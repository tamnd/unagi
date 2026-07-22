package runtime

import (
	"os/user"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func TestPwdModule(t *testing.T) {
	mo, err := ImportModule("pwd")
	if err != nil {
		t.Fatalf("import pwd: %v", err)
	}
	attr := func(name string) objects.Object {
		t.Helper()
		v, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("pwd.%s: %v", name, err)
		}
		return v
	}
	call := func(name string, args ...objects.Object) (objects.Object, error) {
		return objects.Call(attr(name), args)
	}

	// root exists on every POSIX host with uid 0. getpwnam returns a 7-field
	// struct_passwd; only the host-invariant fields are asserted.
	r, err := call("getpwnam", objects.NewStr("root"))
	if err != nil {
		t.Fatalf("getpwnam(root): %v", err)
	}
	if tn := r.TypeName(); tn != "struct_passwd" {
		t.Errorf("getpwnam type = %s, want struct_passwd", tn)
	}
	if n, _ := objects.Len(r); n != 7 {
		t.Errorf("len = %d, want 7", n)
	}
	field := func(v objects.Object, name string) objects.Object {
		x, err := objects.LoadAttr(v, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return x
	}
	if s, _ := objects.AsStr(field(r, "pw_name")); s != "root" {
		t.Errorf("pw_name = %q, want root", s)
	}
	if n, _ := objects.AsInt(field(r, "pw_uid")); n != 0 {
		t.Errorf("pw_uid = %d, want 0", n)
	}

	// getpwuid(0) resolves the same account.
	byuid, err := call("getpwuid", objects.NewInt(0))
	if err != nil {
		t.Fatalf("getpwuid(0): %v", err)
	}
	if s, _ := objects.AsStr(field(byuid, "pw_name")); s != "root" {
		t.Errorf("getpwuid(0).pw_name = %q, want root", s)
	}

	// A missing name and a missing uid both raise KeyError.
	if _, err := call("getpwnam", objects.NewStr("no_such_user_xyzzy")); !isPwdKeyError(err) {
		t.Errorf("getpwnam(missing) = %v, want KeyError", err)
	}
	if _, err := call("getpwuid", objects.NewInt(4294967295)); !isPwdKeyError(err) {
		t.Errorf("getpwuid(missing) = %v, want KeyError", err)
	}

	// The struct_passwd home matches what os/user reports for root.
	if u, uerr := user.Lookup("root"); uerr == nil {
		if s, _ := objects.AsStr(field(r, "pw_dir")); s != u.HomeDir {
			t.Errorf("pw_dir = %q, want %q", s, u.HomeDir)
		}
	}
}

func isPwdKeyError(err error) bool {
	e, ok := err.(*objects.Exception)
	return ok && e.Kind == "KeyError"
}
