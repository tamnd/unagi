package runtime

import (
	"bytes"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// mustCmathAttr imports the cmath module (which registers its functions for
// pickling) and reads one of its members back.
func mustCmathAttr(t *testing.T, name string) objects.Object {
	t.Helper()
	m, err := ImportModule("cmath")
	if err != nil {
		t.Fatalf("import cmath: %v", err)
	}
	o, err := objects.LoadAttr(m, name)
	if err != nil {
		t.Fatalf("cmath.%s: %v", name, err)
	}
	return o
}

// TestCmathFuncPicklesAsModuleGlobal pins that a cmath function pickles as its
// cmath.<name> global, byte for byte against CPython 3.14 at protocol 2 (GLOBAL)
// and protocol 4 (STACK_GLOBAL), the same form CPython derives from the
// function's __module__ and __qualname__. The reference loads back to the one
// function object the module holds.
func TestCmathFuncPicklesAsModuleGlobal(t *testing.T) {
	sqrt := mustCmathAttr(t, "sqrt")
	cases := []struct {
		proto int
		want  []byte
	}{
		{2, []byte("\x80\x02ccmath\nsqrt\nq\x00.")},
		{4, []byte("\x80\x04\x95\x12\x00\x00\x00\x00\x00\x00\x00\x8c\x05cmath\x94\x8c\x04sqrt\x94\x93\x94.")},
	}
	for _, c := range cases {
		data, err := objects.PickleDumps(sqrt, c.proto)
		if err != nil {
			t.Fatalf("dumps(cmath.sqrt, proto=%d): %v", c.proto, err)
		}
		if !bytes.Equal(data, c.want) {
			t.Fatalf("dumps(cmath.sqrt, proto=%d) = %q, want %q", c.proto, data, c.want)
		}
		back, err := objects.PickleLoads(data)
		if err != nil {
			t.Fatalf("loads(cmath.sqrt, proto=%d): %v", c.proto, err)
		}
		if back != sqrt {
			t.Fatalf("round-trip(cmath.sqrt, proto=%d) = %v, want the same cmath.sqrt", c.proto, objects.Repr(back))
		}
	}
}

// TestCmathFloatConstantNotPickledAsGlobal guards the funcObject gate: a cmath
// float constant is not a function, so it must not be registered as a global. It
// pickles as its own value and never references the cmath module.
func TestCmathFloatConstantNotPickledAsGlobal(t *testing.T) {
	pi := mustCmathAttr(t, "pi")
	data, err := objects.PickleDumps(pi, 2)
	if err != nil {
		t.Fatalf("dumps(cmath.pi): %v", err)
	}
	if bytes.Contains(data, []byte("cmath")) {
		t.Fatalf("dumps(cmath.pi) = %q, must not reference the cmath module", data)
	}
	back, err := objects.PickleLoads(data)
	if err != nil {
		t.Fatalf("loads(cmath.pi): %v", err)
	}
	if objects.Repr(back) != objects.Repr(pi) {
		t.Fatalf("round-trip(cmath.pi) = %v, want %v", objects.Repr(back), objects.Repr(pi))
	}
}
