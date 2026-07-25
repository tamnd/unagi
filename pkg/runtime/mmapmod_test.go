//go:build darwin || linux

package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestMmapModule checks the surface and an anonymous read-write round-trip: the
// module imports, exposes the mmap class and the ACCESS_*/PROT_*/MAP_* constants,
// and an anonymous mapping supports write/seek/read, indexing, len and close.
func TestMmapModule(t *testing.T) {
	mo, err := ImportModule("mmap")
	if err != nil {
		t.Fatalf("import mmap: %v", err)
	}
	attr := func(name string) objects.Object {
		t.Helper()
		v, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("mmap.%s: %v", name, err)
		}
		return v
	}
	for _, name := range []string{"ACCESS_READ", "ACCESS_WRITE", "ACCESS_COPY", "ACCESS_DEFAULT", "PROT_READ", "PROT_WRITE", "MAP_SHARED", "MAP_PRIVATE", "MAP_ANON", "PAGESIZE"} {
		if _, ok := objects.AsInt(attr(name)); !ok {
			t.Fatalf("mmap.%s is not an int", name)
		}
	}

	cls := attr("mmap")
	// An anonymous 16-byte read-write mapping.
	mm, err := objects.Call(cls, []objects.Object{objects.NewInt(-1), objects.NewInt(16)})
	if err != nil {
		t.Fatalf("mmap.mmap(-1, 16): %v", err)
	}
	call := func(name string, a ...objects.Object) objects.Object {
		t.Helper()
		v, err := objects.CallMethod(mm, name, a)
		if err != nil {
			t.Fatalf("mm.%s: %v", name, err)
		}
		return v
	}
	// len is 16.
	if n, err := objects.Len(mm); err != nil || n != 16 {
		t.Fatalf("len(mm) = %d, %v; want 16", n, err)
	}
	// write/seek/read round-trip.
	if n, _ := objects.AsInt(call("write", objects.NewBytes([]byte("hello")))); n != 5 {
		t.Fatalf("write returned %d, want 5", n)
	}
	call("seek", objects.NewInt(0))
	got, err := objects.CallMethod(mm, "read", []objects.Object{objects.NewInt(5)})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if b, ok := objects.AsBytesLike(got); !ok || string(b) != "hello" {
		t.Fatalf("read returned %v, want b'hello'", got)
	}
	// index returns an int byte.
	item, err := objects.GetItem(mm, objects.NewInt(0))
	if err != nil {
		t.Fatalf("mm[0]: %v", err)
	}
	if n, _ := objects.AsInt(item); n != 'h' {
		t.Fatalf("mm[0] = %d, want %d", n, 'h')
	}
	call("close")
	if closed, err := objects.LoadAttr(mm, "closed"); err != nil || closed != objects.True {
		t.Fatalf("mm.closed = %v, %v; want True", closed, err)
	}
}
