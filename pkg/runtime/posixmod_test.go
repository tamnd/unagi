package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func TestPosixSkeleton(t *testing.T) {
	mo, err := ImportModule("posix")
	if err != nil {
		t.Fatalf("import posix: %v", err)
	}
	attr := func(name string) objects.Object {
		t.Helper()
		v, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("posix.%s: %v", name, err)
		}
		return v
	}

	// error is the OSError class object.
	oserr, ok := objects.ExcClassValue("OSError")
	if !ok {
		t.Fatal("OSError class missing")
	}
	if attr("error") != oserr {
		t.Error("posix.error is not OSError")
	}

	// The access-mode constants are POSIX-universal, asserted by value.
	access := map[string]int64{"F_OK": 0, "R_OK": 4, "W_OK": 2, "X_OK": 1}
	for name, want := range access {
		if n, ok := objects.AsInt(attr(name)); !ok || n != want {
			t.Errorf("posix.%s = %v, want %d", name, attr(name), want)
		}
	}

	// The open flags are platform-specific, so only their shape is checked: each
	// is an int and the three access modes are distinct.
	flagNames := []string{"O_RDONLY", "O_WRONLY", "O_RDWR", "O_APPEND", "O_CREAT",
		"O_EXCL", "O_TRUNC", "O_NONBLOCK", "O_CLOEXEC", "O_DIRECTORY", "O_NOFOLLOW"}
	for _, name := range flagNames {
		if _, ok := objects.AsInt(attr(name)); !ok {
			t.Errorf("posix.%s is not an int", name)
		}
	}
	rd, _ := objects.AsInt(attr("O_RDONLY"))
	wr, _ := objects.AsInt(attr("O_WRONLY"))
	rw, _ := objects.AsInt(attr("O_RDWR"))
	if rd == wr || wr == rw || rd == rw {
		t.Error("O_RDONLY/O_WRONLY/O_RDWR should be distinct")
	}

	call := func(name string, args ...objects.Object) objects.Object {
		t.Helper()
		v, err := objects.Call(attr(name), args)
		if err != nil {
			t.Fatalf("posix.%s: %v", name, err)
		}
		return v
	}

	if n, ok := objects.AsInt(call("getpid")); !ok || n <= 0 {
		t.Errorf("getpid = %v, want positive", call("getpid"))
	}
	if n, ok := objects.AsInt(call("getppid")); !ok || n <= 0 {
		t.Errorf("getppid = %v, want positive", call("getppid"))
	}
	if _, ok := objects.AsStr(call("getcwd")); !ok {
		t.Error("getcwd should return str")
	}
	if _, ok := objects.AsStr(call("strerror", objects.NewInt(2))); !ok {
		t.Error("strerror should return str")
	}

	// umask sets and returns the prior mask; set a known value then read it back
	// so the assertion does not depend on the ambient mask, and restore it.
	prev := call("umask", objects.NewInt(0o22))
	cur, ok := objects.AsInt(call("umask", prev))
	if !ok || cur != 0o22 {
		t.Errorf("umask roundtrip = %v, want 0o22", cur)
	}
	call("umask", prev)

	// _have_functions is a list, environ is a dict, _create_environ returns a
	// fresh dict distinct from environ.
	if tn := attr("_have_functions").TypeName(); tn != "list" {
		t.Errorf("_have_functions is %s, want list", tn)
	}
	fresh := call("_create_environ")
	if fresh == attr("environ") {
		t.Error("_create_environ should return a fresh dict")
	}

	// A bad argument count raises TypeError rather than panicking.
	if _, err := objects.Call(attr("getpid"), []objects.Object{objects.NewInt(1)}); err == nil {
		t.Error("getpid(1) should raise")
	}

	// cpu_count and getuid/geteuid return ints; cpu_count is at least one.
	if n, ok := objects.AsInt(call("cpu_count")); !ok || n < 1 {
		t.Errorf("cpu_count = %v, want >= 1", call("cpu_count"))
	}
	if _, ok := objects.AsInt(call("getuid")); !ok {
		t.Error("getuid should return int")
	}
	if _, ok := objects.AsInt(call("geteuid")); !ok {
		t.Error("geteuid should return int")
	}

	// __all__ is the synthesized public surface os.py's _get_exports_list reads
	// (CPython's posix has none and relies on dir(), which unagi lacks). It is a
	// non-empty list of public names and never leaks an underscore name.
	all := attr("__all__")
	if tn := all.TypeName(); tn != "list" {
		t.Fatalf("__all__ is %s, want list", tn)
	}
	n, err := objects.Len(all)
	if err != nil {
		t.Fatalf("len(__all__): %v", err)
	}
	if n == 0 {
		t.Error("__all__ is empty")
	}
	sawGetcwd := false
	for i := 0; i < n; i++ {
		it, err := objects.GetItem(all, objects.NewInt(int64(i)))
		if err != nil {
			t.Fatalf("__all__[%d]: %v", i, err)
		}
		s, ok := objects.AsStr(it)
		if !ok {
			t.Errorf("__all__ has non-str %v", it)
			continue
		}
		if s == "" || s[0] == '_' {
			t.Errorf("__all__ leaks private name %q", s)
		}
		if s == "getcwd" {
			sawGetcwd = true
		}
	}
	if !sawGetcwd {
		t.Error("__all__ missing getcwd")
	}
}
