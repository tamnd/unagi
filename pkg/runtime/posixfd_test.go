package runtime

import (
	"path/filepath"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// fdIntFn returns a helper that unwraps a call's (result, error) pair to a Go
// int, failing the test on error. It is a closure so it can be applied straight
// to a two-value call: intOf(posixOpen(...)).
func fdIntFn(t *testing.T) func(objects.Object, error) int {
	return func(o objects.Object, err error) int {
		t.Helper()
		if err != nil {
			t.Fatalf("call: %v", err)
		}
		v, ok := objects.AsInt(o)
		if !ok {
			t.Fatalf("result %v is not an int", o)
		}
		return int(v)
	}
}

// TestPosixFDRoundTrip opens a file, writes, seeks and reads it back, exercising
// open/write/lseek/read/close/fstat/ftruncate/fsync end to end.
func TestPosixFDRoundTrip(t *testing.T) {
	intOf := fdIntFn(t)
	path := filepath.Join(t.TempDir(), "data.bin")
	flags, _ := objects.AsInt(mustAttr(t, "O_CREAT"))
	wr, _ := objects.AsInt(mustAttr(t, "O_WRONLY"))
	tr, _ := objects.AsInt(mustAttr(t, "O_TRUNC"))
	rd, _ := objects.AsInt(mustAttr(t, "O_RDONLY"))
	rdwr, _ := objects.AsInt(mustAttr(t, "O_RDWR"))

	fd := intOf(posixOpen([]objects.Object{
		objects.NewStr(path), objects.NewInt(flags | wr | tr), objects.NewInt(0o644),
	}))
	n := intOf(posixWrite([]objects.Object{objects.NewInt(int64(fd)), objects.NewBytes([]byte("hello world"))}))
	if n != 11 {
		t.Fatalf("write = %d, want 11", n)
	}
	if tty, _ := posixIsatty([]objects.Object{objects.NewInt(int64(fd))}); objects.Truth(tty) {
		t.Fatal("isatty(file) = true")
	}
	if _, err := posixClose([]objects.Object{objects.NewInt(int64(fd))}); err != nil {
		t.Fatal(err)
	}

	fd = intOf(posixOpen([]objects.Object{objects.NewStr(path), objects.NewInt(rd)}))
	if off := intOf(posixLseek([]objects.Object{objects.NewInt(int64(fd)), objects.NewInt(6), objects.NewInt(0)})); off != 6 {
		t.Fatalf("lseek = %d, want 6", off)
	}
	got, err := posixRead([]objects.Object{objects.NewInt(int64(fd)), objects.NewInt(5)})
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := objects.AsBytes(got); string(b) != "world" {
		t.Fatalf("read = %q, want world", b)
	}
	_, _ = posixClose([]objects.Object{objects.NewInt(int64(fd))})

	// truncate and confirm the new size through fstat
	fd = intOf(posixOpen([]objects.Object{objects.NewStr(path), objects.NewInt(rdwr)}))
	if _, err := posixFtruncate([]objects.Object{objects.NewInt(int64(fd)), objects.NewInt(5)}); err != nil {
		t.Fatal(err)
	}
	if _, err := posixFsync([]objects.Object{objects.NewInt(int64(fd))}); err != nil {
		t.Fatal(err)
	}
	st, err := posixFstat([]objects.Object{objects.NewInt(int64(fd))})
	if err != nil {
		t.Fatal(err)
	}
	if size, _ := objects.AsInt(statAttr(t, st, "st_size")); size != 5 {
		t.Fatalf("st_size after truncate = %d, want 5", size)
	}
	_, _ = posixClose([]objects.Object{objects.NewInt(int64(fd))})
}

// TestPosixFDDup covers dup, dup2 and pipe: a byte written to the dup2 target
// comes out the pipe's read end.
func TestPosixFDDup(t *testing.T) {
	intOf := fdIntFn(t)
	pipe, err := posixPipe(nil)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := objects.AsInt(mustItem(t, pipe, 0))
	w, _ := objects.AsInt(mustItem(t, pipe, 1))

	dupW := intOf(posixDup([]objects.Object{objects.NewInt(w)}))
	if dupW == int(w) {
		t.Fatal("dup returned the same fd")
	}
	_, _ = posixClose([]objects.Object{objects.NewInt(int64(dupW))})

	target := intOf(posixDup2([]objects.Object{objects.NewInt(w), objects.NewInt(15)}))
	if target != 15 {
		t.Fatalf("dup2 target = %d, want 15", target)
	}
	if _, err := posixWrite([]objects.Object{objects.NewInt(int64(target)), objects.NewBytes([]byte("pi"))}); err != nil {
		t.Fatal(err)
	}
	got, err := posixRead([]objects.Object{objects.NewInt(r), objects.NewInt(2)})
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := objects.AsBytes(got); string(b) != "pi" {
		t.Fatalf("pipe read = %q, want pi", b)
	}
	for _, fd := range []int64{r, w, 15} {
		_, _ = posixClose([]objects.Object{objects.NewInt(fd)})
	}
}

// TestPosixOpenMissing maps a missing path to FileNotFoundError.
func TestPosixOpenMissing(t *testing.T) {
	rd, _ := objects.AsInt(mustAttr(t, "O_RDONLY"))
	_, err := posixOpen([]objects.Object{
		objects.NewStr(filepath.Join(t.TempDir(), "nope")), objects.NewInt(rd),
	})
	exc, ok := err.(*objects.Exception)
	if !ok {
		t.Fatalf("err = %v, want Exception", err)
	}
	if exc.Kind != "FileNotFoundError" {
		t.Fatalf("kind = %s, want FileNotFoundError", exc.Kind)
	}
}

// mustAttr reads a posix open flag by name from the shared flag table.
func mustAttr(t *testing.T, name string) objects.Object {
	t.Helper()
	for _, f := range posixOpenFlags {
		if f.name == name {
			return objects.NewInt(int64(f.val))
		}
	}
	t.Fatalf("posix has no %s", name)
	return nil
}

func mustItem(t *testing.T, o objects.Object, i int) objects.Object {
	t.Helper()
	v, err := objects.GetItem(o, objects.NewInt(int64(i)))
	if err != nil {
		t.Fatalf("item %d: %v", i, err)
	}
	return v
}
