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
	for _, f := range []string{"st_ino=", "st_size=", "st_mtime=", "st_atime_ns="} {
		if !strings.Contains(r, f) {
			t.Fatalf("repr %q missing %q", r, f)
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
