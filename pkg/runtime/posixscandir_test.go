package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestDirEntryQueries builds DirEntry values over a temp tree and checks the
// type queries and stat answer from the entry path.
func TestDirEntryQueries(t *testing.T) {
	cls, err := buildPosixDirEntry()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	mk := func(name string) objects.Object {
		e, err := objects.Call(cls, []objects.Object{
			objects.NewStr(name), objects.NewStr(filepath.Join(dir, name)),
		})
		if err != nil {
			t.Fatalf("build DirEntry %q: %v", name, err)
		}
		return e
	}

	sub := mk("sub")
	if got := callBool(t, sub, "is_dir"); !got {
		t.Fatal("sub.is_dir() = false")
	}
	if got := callBool(t, sub, "is_file"); got {
		t.Fatal("sub.is_file() = true")
	}
	if got := callBool(t, sub, "is_symlink"); got {
		t.Fatal("sub.is_symlink() = true")
	}
	if got := callBool(t, sub, "is_junction"); got {
		t.Fatal("sub.is_junction() = true")
	}
	st := callMethod(t, sub, "stat")
	if n := statLen(t, st); n != 10 {
		t.Fatalf("stat len = %d", n)
	}

	f := mk("f.txt")
	if got := callBool(t, f, "is_file"); !got {
		t.Fatal("f.is_file() = false")
	}
	if got := callBool(t, f, "is_dir"); got {
		t.Fatal("f.is_dir() = true")
	}
	fst := callMethod(t, f, "stat")
	if size, ok := objects.AsInt(statAttr(t, fst, "st_size")); !ok || size != 2 {
		t.Fatalf("f stat st_size = %v, want 2", size)
	}
	if ino, ok := objects.AsInt(callMethod(t, f, "inode")); !ok || ino <= 0 {
		t.Fatalf("f.inode() = %v", ino)
	}

	// A vanished entry reports is_dir/is_file false rather than raising.
	gone := mk("gone")
	if got := callBool(t, gone, "is_dir"); got {
		t.Fatal("gone.is_dir() = true")
	}
	if got := callBool(t, gone, "is_file"); got {
		t.Fatal("gone.is_file() = true")
	}
}

// TestPosixScandir drives posixScandir over a temp dir and checks the entry
// names round-trip.
func TestPosixScandir(t *testing.T) {
	de, err := buildPosixDirEntry()
	if err != nil {
		t.Fatal(err)
	}
	sc, err := buildPosixScandir()
	if err != nil {
		t.Fatal(err)
	}
	posixDirEntryClass, posixScandirClass = de, sc

	dir := t.TempDir()
	for _, n := range []string{"a", "b", "c"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	it, err := posixScandir([]objects.Object{objects.NewStr(dir)})
	if err != nil {
		t.Fatalf("scandir: %v", err)
	}
	got := map[string]bool{}
	for {
		e, err := objects.CallMethod(it, "__next__", nil)
		if err != nil {
			if exc, ok := err.(*objects.Exception); ok && exc.Kind == "StopIteration" {
				break
			}
			t.Fatalf("__next__: %v", err)
		}
		name, _ := objects.AsStr(statAttr(t, e, "name"))
		got[name] = true
	}
	for _, n := range []string{"a", "b", "c"} {
		if !got[n] {
			t.Fatalf("scandir missing %q (got %v)", n, got)
		}
	}
	if len(got) != 3 {
		t.Fatalf("scandir extra entries: %v", got)
	}
}

func callMethod(t *testing.T, o objects.Object, name string) objects.Object {
	t.Helper()
	v, err := objects.CallMethod(o, name, nil)
	if err != nil {
		t.Fatalf("%s(): %v", name, err)
	}
	return v
}

func callBool(t *testing.T, o objects.Object, name string) bool {
	t.Helper()
	return objects.Truth(callMethod(t, o, name))
}
