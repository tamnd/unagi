package runtime

import (
	"bytes"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// mustMathAttr imports the math module (which registers its functions for
// pickling) and reads one of its members back.
func mustMathAttr(t *testing.T, name string) objects.Object {
	t.Helper()
	m, err := ImportModule("math")
	if err != nil {
		t.Fatalf("import math: %v", err)
	}
	o, err := objects.LoadAttr(m, name)
	if err != nil {
		t.Fatalf("math.%s: %v", name, err)
	}
	return o
}

// TestMathFuncPicklesAsModuleGlobal pins that a math function pickles as its
// math.<name> global, byte for byte against CPython 3.14: protocol 2 writes the
// name as a GLOBAL, protocol 4 as a STACK_GLOBAL, the same form CPython derives
// from the function's __module__ and __qualname__. The reference loads back to
// the one function object the module holds.
func TestMathFuncPicklesAsModuleGlobal(t *testing.T) {
	sqrt := mustMathAttr(t, "sqrt")
	cases := []struct {
		proto int
		want  []byte
	}{
		{2, []byte("\x80\x02cmath\nsqrt\nq\x00.")},
		{4, []byte("\x80\x04\x95\x11\x00\x00\x00\x00\x00\x00\x00\x8c\x04math\x94\x8c\x04sqrt\x94\x93\x94.")},
	}
	for _, c := range cases {
		data, err := objects.PickleDumps(sqrt, c.proto)
		if err != nil {
			t.Fatalf("dumps(math.sqrt, proto=%d): %v", c.proto, err)
		}
		if !bytes.Equal(data, c.want) {
			t.Fatalf("dumps(math.sqrt, proto=%d) = %q, want %q", c.proto, data, c.want)
		}
		back, err := objects.PickleLoads(data)
		if err != nil {
			t.Fatalf("loads(math.sqrt, proto=%d): %v", c.proto, err)
		}
		if back != sqrt {
			t.Fatalf("round-trip(math.sqrt, proto=%d) = %v, want the same math.sqrt", c.proto, objects.Repr(back))
		}
	}
}

// TestMathConstantNotPickledAsGlobal guards the funcObject gate: a math constant
// is a plain float, not a function, so it must not be registered as a global. It
// pickles as its own value and never references the math module.
func TestMathConstantNotPickledAsGlobal(t *testing.T) {
	pi := mustMathAttr(t, "pi")
	data, err := objects.PickleDumps(pi, 2)
	if err != nil {
		t.Fatalf("dumps(math.pi): %v", err)
	}
	if bytes.Contains(data, []byte("math")) {
		t.Fatalf("dumps(math.pi) = %q, must not reference the math module", data)
	}
	back, err := objects.PickleLoads(data)
	if err != nil {
		t.Fatalf("loads(math.pi): %v", err)
	}
	if objects.Repr(back) != objects.Repr(pi) {
		t.Fatalf("round-trip(math.pi) = %v, want %v", objects.Repr(back), objects.Repr(pi))
	}
}
