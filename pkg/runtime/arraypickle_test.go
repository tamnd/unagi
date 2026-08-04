package runtime

import (
	"encoding/hex"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// mustArray imports the array module (which registers array.array and
// _array_reconstructor for pickling) and builds array.array(typecode, init).
func mustArray(t *testing.T, typecode string, init objects.Object) objects.Object {
	t.Helper()
	if _, err := ImportModule("array"); err != nil {
		t.Fatalf("import array: %v", err)
	}
	a, err := objects.NewArray(objects.NewStr(typecode), init)
	if err != nil {
		t.Fatalf("NewArray(%q): %v", typecode, err)
	}
	return a
}

// TestArrayPickleDumps pins the bytes array.array('i', [1, 2, 3]) pickles to,
// byte for byte against CPython 3.14: protocol 2 names the array type as a GLOBAL
// applied to (typecode, list), and protocol 5 names _array_reconstructor applied
// to the machine-format tuple, each builtin resolved through the pickle registry.
func TestArrayPickleDumps(t *testing.T) {
	a := mustArray(t, "i", objects.NewList([]objects.Object{
		objects.NewInt(1), objects.NewInt(2), objects.NewInt(3),
	}))
	cases := []struct {
		proto int
		want  string
	}{
		{2, "80026361727261790a61727261790a710058010000006971015d7102284b014b024b03658671035271042e"},
		{5, "8005954e000000000000008c056172726179948c145f61727261795f7265636f6e7374727563746f72949394288c056172726179948c0561727261799493948c0169944b08430c01000000020000000300000094749452942e"},
	}
	for _, tc := range cases {
		got, err := objects.PickleDumps(a, tc.proto)
		if err != nil {
			t.Fatalf("PickleDumps(proto=%d): %v", tc.proto, err)
		}
		if h := hex.EncodeToString(got); h != tc.want {
			t.Fatalf("PickleDumps(proto=%d)\n got  %s\n want %s", tc.proto, h, tc.want)
		}
	}
}

// TestArrayPickleRoundTrip confirms an array survives dumps/loads at every binary
// protocol, coming back an array.array with the same typecode and elements, for
// both the legacy list reduction (protocols 2 and 3) and the machine-format
// reduction (protocols 3 and up).
func TestArrayPickleRoundTrip(t *testing.T) {
	a := mustArray(t, "d", objects.NewList([]objects.Object{
		objects.NewFloat(1.5), objects.NewFloat(-2.25), objects.NewFloat(0.0),
	}))
	for _, proto := range []int{2, 3, 4, 5} {
		data, err := objects.PickleDumps(a, proto)
		if err != nil {
			t.Fatalf("dumps(proto=%d): %v", proto, err)
		}
		back, err := objects.PickleLoads(data)
		if err != nil {
			t.Fatalf("loads(proto=%d): %v", proto, err)
		}
		if back.TypeName() != "array.array" {
			t.Fatalf("loads(proto=%d) = %s, want array.array", proto, back.TypeName())
		}
		eqObj, err := objects.Compare(objects.OpEq, back, a)
		if err != nil {
			t.Fatalf("compare(proto=%d): %v", proto, err)
		}
		if eq, _ := objects.TruthOf(eqObj); !eq {
			t.Fatalf("loads(proto=%d) = %s, want %s", proto, objects.Repr(back), objects.Repr(a))
		}
	}
}

// TestArrayPickleTypeGlobal confirms the array type and the reconstructor pickle
// as bare global references that resolve back to the very same builtin objects,
// so a stream that names them recovers the runtime's own callables.
func TestArrayPickleTypeGlobal(t *testing.T) {
	m, err := ImportModule("array")
	if err != nil {
		t.Fatalf("import array: %v", err)
	}
	for _, name := range []string{"array", "_array_reconstructor"} {
		want, err := objects.LoadAttr(m, name)
		if err != nil {
			t.Fatalf("attr %s: %v", name, err)
		}
		data, err := objects.PickleDumps(want, 4)
		if err != nil {
			t.Fatalf("dumps(%s): %v", name, err)
		}
		back, err := objects.PickleLoads(data)
		if err != nil {
			t.Fatalf("loads(%s): %v", name, err)
		}
		if back != want {
			t.Fatalf("loads(%s) = %s, want the registered builtin", name, objects.Repr(back))
		}
	}
}
