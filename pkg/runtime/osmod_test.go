package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func TestOSCore(t *testing.T) {
	mo, err := ImportModule("os")
	if err != nil {
		t.Fatalf("import os: %v", err)
	}
	attr := func(name string) objects.Object {
		v, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("os.%s: %v", name, err)
		}
		return v
	}
	if s, _ := objects.AsStr(attr("name")); s != "posix" {
		t.Errorf("os.name = %q, want posix", s)
	}
	if s, _ := objects.AsStr(attr("sep")); s != "/" {
		t.Errorf("os.sep = %q, want /", s)
	}
	if attr("altsep") != objects.None {
		t.Errorf("os.altsep = %v, want None", attr("altsep"))
	}
	pid, err := objects.Call(attr("getpid"), nil)
	if err != nil {
		t.Fatalf("getpid: %v", err)
	}
	if n, _ := objects.AsInt(pid); n <= 0 {
		t.Errorf("os.getpid() = %d, want positive", n)
	}
}

func TestOSGetenv(t *testing.T) {
	mo, err := ImportModule("os")
	if err != nil {
		t.Fatalf("import os: %v", err)
	}
	getenv, _ := objects.LoadAttr(mo, "getenv")
	miss, err := objects.Call(getenv, []objects.Object{objects.NewStr("UNAGI_NOPE_XYZ")})
	if err != nil || miss != objects.None {
		t.Errorf("getenv missing = %v, %v, want None, nil", miss, err)
	}
	def, _ := objects.Call(getenv, []objects.Object{objects.NewStr("UNAGI_NOPE_XYZ"), objects.NewStr("fb")})
	if s, _ := objects.AsStr(def); s != "fb" {
		t.Errorf("getenv default = %q, want fb", s)
	}
}

func TestOSListdir(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mo, err := ImportModule("os")
	if err != nil {
		t.Fatalf("import os: %v", err)
	}
	listdir, _ := objects.LoadAttr(mo, "listdir")
	got, err := objects.Call(listdir, []objects.Object{objects.NewStr(dir)})
	if err != nil {
		t.Fatalf("listdir: %v", err)
	}
	if repr := objects.Repr(got); repr != "['a.txt', 'b.txt']" {
		t.Errorf("listdir = %s, want ['a.txt', 'b.txt']", repr)
	}
}
