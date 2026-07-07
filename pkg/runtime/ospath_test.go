package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func osPathFn(t *testing.T, name string) objects.Object {
	t.Helper()
	mo, err := ImportModule("os.path")
	if err != nil {
		t.Fatalf("import os.path: %v", err)
	}
	fn, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("os.path.%s: %v", name, err)
	}
	return fn
}

func strResult(t *testing.T, fn objects.Object, args ...objects.Object) string {
	t.Helper()
	v, err := objects.Call(fn, args)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	s, _ := objects.AsStr(v)
	return s
}

func TestOSPathJoinNormpath(t *testing.T) {
	join := osPathFn(t, "join")
	cases := []struct {
		args []objects.Object
		want string
	}{
		{[]objects.Object{objects.NewStr("a"), objects.NewStr("b"), objects.NewStr("c")}, "a/b/c"},
		{[]objects.Object{objects.NewStr("a"), objects.NewStr("/b")}, "/b"},
		{[]objects.Object{objects.NewStr("a/"), objects.NewStr("b")}, "a/b"},
	}
	for _, c := range cases {
		if got := strResult(t, join, c.args...); got != c.want {
			t.Errorf("join%v = %q, want %q", c.args, got, c.want)
		}
	}
	norm := osPathFn(t, "normpath")
	for in, want := range map[string]string{
		"a/./b/../c":   "a/c",
		"a//b":         "a/b",
		"//a/b":        "//a/b",
		"///a/b":       "/a/b",
		"":             ".",
		"a/b/../../..": "..",
	} {
		if got := strResult(t, norm, objects.NewStr(in)); got != want {
			t.Errorf("normpath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOSPathSplitext(t *testing.T) {
	splitext := osPathFn(t, "splitext")
	cases := []struct{ in, root, ext string }{
		{"a/b.txt", "a/b", ".txt"},
		{"a/b", "a/b", ""},
		{".bashrc", ".bashrc", ""},
		{"a.tar.gz", "a.tar", ".gz"},
	}
	for _, c := range cases {
		v, err := objects.Call(splitext, []objects.Object{objects.NewStr(c.in)})
		if err != nil {
			t.Fatalf("splitext(%q): %v", c.in, err)
		}
		if got := objects.Repr(v); got != "('"+c.root+"', '"+c.ext+"')" {
			t.Errorf("splitext(%q) = %s", c.in, got)
		}
	}
}

func TestOSPathProbes(t *testing.T) {
	exists := osPathFn(t, "exists")
	isdir := osPathFn(t, "isdir")
	isfile := osPathFn(t, "isfile")
	check := func(fn objects.Object, path string) bool {
		v, err := objects.Call(fn, []objects.Object{objects.NewStr(path)})
		if err != nil {
			t.Fatalf("probe %q: %v", path, err)
		}
		return v == objects.True
	}
	if !check(exists, "/") || !check(isdir, "/") || check(isfile, "/") {
		t.Error("root probes wrong")
	}
	if check(exists, "/no/such/unagi/path") {
		t.Error("missing path reported as existing")
	}
}

func TestOSPathAttachedToOS(t *testing.T) {
	osMod, err := ImportModule("os")
	if err != nil {
		t.Fatalf("import os: %v", err)
	}
	pathMod, err := objects.LoadAttr(osMod, "path")
	if err != nil {
		t.Fatalf("os.path attribute: %v", err)
	}
	join, err := objects.LoadAttr(pathMod, "join")
	if err != nil {
		t.Fatalf("os.path.join: %v", err)
	}
	if got := strResult(t, join, objects.NewStr("x"), objects.NewStr("y")); got != "x/y" {
		t.Errorf("os.path.join = %q, want x/y", got)
	}
}
