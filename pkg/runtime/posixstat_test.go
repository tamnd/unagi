//go:build !windows

package runtime

import (
	"os"
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestStatResultShape checks the structseq stat build: the visible sequence is
// the ten-int view, the named time attributes are floats distinct from the int
// seconds in the tuple, and the nanosecond attributes are ints.
func TestStatResultShape(t *testing.T) {
	f, err := os.CreateTemp("", "unagi-stat-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := f.WriteString("hello"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	st, err := posixStat([]objects.Object{objects.NewStr(f.Name())})
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if n := statLen(t, st); n != 10 {
		t.Fatalf("len = %d, want 10", n)
	}
	if size, ok := objects.AsInt(statAttr(t, st, "st_size")); !ok || size != 5 {
		t.Fatalf("st_size = %v, want 5", size)
	}
	// The tuple slot 7 is the int seconds; st_atime is the float form.
	slot7, ok := objects.AsInt(statItem(t, st, 7))
	if !ok {
		t.Fatal("st[7] is not an int")
	}
	af, ok := objects.AsFloat(statAttr(t, st, "st_atime"))
	if !ok {
		t.Fatal("st_atime is not a float")
	}
	if int64(af) != slot7 {
		t.Fatalf("int(st_atime)=%d != st[7]=%d", int64(af), slot7)
	}
	if _, ok := objects.AsInt(statAttr(t, st, "st_atime_ns")); !ok {
		t.Fatal("st_atime_ns is not an int")
	}
}

// TestStatResultTypeAttrs checks the class-object surface os.py leans on.
func TestStatResultTypeAttrs(t *testing.T) {
	if name, _ := objects.AsStr(statAttr(t, posixStatResultType, "__name__")); name != "stat_result" {
		t.Fatalf("__name__ = %q", name)
	}
	if n, _ := objects.AsInt(statAttr(t, posixStatResultType, "n_sequence_fields")); n != 10 {
		t.Fatalf("n_sequence_fields = %d, want 10", n)
	}
	if n, _ := objects.AsInt(statAttr(t, posixStatResultType, "n_unnamed_fields")); n != 3 {
		t.Fatalf("n_unnamed_fields = %d, want 3", n)
	}
	// n_fields is the full named count, common plus the platform extras.
	want := int64(len(posixStatCommonFields) + len(posixStatExtraNames))
	if n, _ := objects.AsInt(statAttr(t, posixStatResultType, "n_fields")); n != want {
		t.Fatalf("n_fields = %d, want %d", n, want)
	}
}

// TestStatResultRepr checks the structseq repr spells every named field. The
// exact fields are platform-specific, so this only asserts the shared prefix
// and a couple of always-present fields rather than the full string.
func TestStatResultRepr(t *testing.T) {
	// Stat a real path so the normalized struct carries the platform extras the
	// type's field list expects; a hand-built statNormal would be short on hosts
	// that add fields past the common set.
	st, err := posixStat([]objects.Object{objects.NewStr(os.TempDir())})
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	r, err := objects.ReprE(st)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(r, "os.stat_result(st_mode=") {
		t.Fatalf("repr = %q", r)
	}
	for _, f := range []string{"st_ino=", "st_size=", "st_mtime="} {
		if !strings.Contains(r, f) {
			t.Fatalf("repr %q missing %q", r, f)
		}
	}
	// CPython's structseq repr shows only the n_sequence_fields visible members,
	// so the named-only nanosecond fields stay out of the repr.
	if strings.Contains(r, "st_atime_ns=") {
		t.Fatalf("repr %q should not list the named-only st_atime_ns", r)
	}
}

// TestStatArgTypes checks os.stat/lstat/access accept the argument types CPython
// does: a bytes path everywhere, an integer file descriptor for stat only, and a
// float rejected with the type-specific message. os.PathLike is exercised end to
// end by the conformance fixture, which reduces __fspath__ through the same
// posixFsPath helper.
func TestStatArgTypes(t *testing.T) {
	f, err := os.CreateTemp("", "unagi-statargs-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := f.WriteString("hello"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	name := f.Name()

	// A bytes path stats the same file a str path does.
	st, err := posixStat([]objects.Object{objects.NewBytes([]byte(name))})
	if err != nil {
		t.Fatalf("stat(bytes): %v", err)
	}
	if size, ok := objects.AsInt(statAttr(t, st, "st_size")); !ok || size != 5 {
		t.Fatalf("stat(bytes) st_size = %v, want 5", size)
	}

	// An integer file descriptor stats through fstat: stat on the open fd reports
	// the same size.
	fd, oerr := os.Open(name)
	if oerr != nil {
		t.Fatal(oerr)
	}
	defer func() { _ = fd.Close() }()
	stFd, err := posixStat([]objects.Object{objects.NewInt(int64(fd.Fd()))})
	if err != nil {
		t.Fatalf("stat(fd): %v", err)
	}
	if size, ok := objects.AsInt(statAttr(t, stFd, "st_size")); !ok || size != 5 {
		t.Fatalf("stat(fd) st_size = %v, want 5", size)
	}

	// A bytes path also works through lstat and access.
	if _, err := posixLstat([]objects.Object{objects.NewBytes([]byte(name))}); err != nil {
		t.Fatalf("lstat(bytes): %v", err)
	}
	acc, err := posixAccess([]objects.Object{objects.NewBytes([]byte(name)), objects.NewInt(0)})
	if err != nil {
		t.Fatalf("access(bytes): %v", err)
	}
	if b, _ := objects.TruthOf(acc); !b {
		t.Errorf("access(bytes, F_OK) = false, want true")
	}

	// A float is a TypeError with the call's own message; lstat has no fd form, so
	// an integer is a TypeError there too.
	for _, tc := range []struct {
		name    string
		call    func() (objects.Object, error)
		wantSub string
	}{
		{"stat(float)", func() (objects.Object, error) { return posixStat([]objects.Object{objects.NewFloat(1.5)}) },
			"stat: path should be string, bytes, os.PathLike or integer, not float"},
		{"lstat(float)", func() (objects.Object, error) { return posixLstat([]objects.Object{objects.NewFloat(1.5)}) },
			"lstat: path should be string, bytes or os.PathLike, not float"},
		{"lstat(int)", func() (objects.Object, error) { return posixLstat([]objects.Object{objects.NewInt(0)}) },
			"lstat: path should be string, bytes or os.PathLike, not int"},
		{"access(float)", func() (objects.Object, error) {
			return posixAccess([]objects.Object{objects.NewFloat(1.5), objects.NewInt(0)})
		}, "access: path should be string, bytes or os.PathLike, not float"},
	} {
		_, err := tc.call()
		if err == nil {
			t.Errorf("%s did not raise", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.wantSub) {
			t.Errorf("%s error = %q, want substring %q", tc.name, err.Error(), tc.wantSub)
		}
	}
}

func statAttr(t *testing.T, o objects.Object, name string) objects.Object {
	t.Helper()
	v, err := objects.LoadAttr(o, name)
	if err != nil {
		t.Fatalf("LoadAttr %q: %v", name, err)
	}
	return v
}

func statItem(t *testing.T, o objects.Object, i int) objects.Object {
	t.Helper()
	v, err := objects.GetItem(o, objects.NewInt(int64(i)))
	if err != nil {
		t.Fatalf("GetItem %d: %v", i, err)
	}
	return v
}

func statLen(t *testing.T, o objects.Object) int {
	t.Helper()
	n, err := objects.Len(o)
	if err != nil {
		t.Fatalf("Len: %v", err)
	}
	return n
}
