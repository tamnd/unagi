package runtime

import (
	"bytes"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestOpenPicklesAsIOGlobal pins that the builtin open pickles as the _io.open
// global, the qualified name CPython writes because open.__module__ is _io rather
// than builtins. The bytes match at protocol 2 (GLOBAL) and protocol 4
// (STACK_GLOBAL), and the reference loads back to the one open function object.
func TestOpenPicklesAsIOGlobal(t *testing.T) {
	open, ok := Builtin("open")
	if !ok {
		t.Fatal("Builtin(open): not found")
	}
	cases := []struct {
		proto int
		want  []byte
	}{
		{2, []byte("\x80\x02c_io\nopen\nq\x00.")},
		{4, []byte("\x80\x04\x95\x10\x00\x00\x00\x00\x00\x00\x00\x8c\x03_io\x94\x8c\x04open\x94\x93\x94.")},
	}
	for _, c := range cases {
		data, err := objects.PickleDumps(open, c.proto)
		if err != nil {
			t.Fatalf("dumps(open, proto=%d): %v", c.proto, err)
		}
		if !bytes.Equal(data, c.want) {
			t.Fatalf("dumps(open, proto=%d) = %q, want %q", c.proto, data, c.want)
		}
		back, err := objects.PickleLoads(data)
		if err != nil {
			t.Fatalf("loads(open, proto=%d): %v", c.proto, err)
		}
		if back != open {
			t.Fatalf("round-trip(open, proto=%d) = %v, want the same open function", c.proto, back)
		}
	}
}

// TestOpenNotPickledAsBuiltinsGlobal guards the namer exclusion: open must not
// leak out as builtins.open. Loading that stand-in bytes would not resolve to the
// open function, so the only accepted reference is _io.open.
func TestOpenNotPickledAsBuiltinsGlobal(t *testing.T) {
	open, ok := Builtin("open")
	if !ok {
		t.Fatal("Builtin(open): not found")
	}
	data, err := objects.PickleDumps(open, 4)
	if err != nil {
		t.Fatalf("dumps(open): %v", err)
	}
	if bytes.Contains(data, []byte("builtins")) {
		t.Fatalf("dumps(open) = %q, must not reference the builtins module", data)
	}
	if !bytes.Contains(data, []byte("_io")) {
		t.Fatalf("dumps(open) = %q, want the _io module", data)
	}
}
