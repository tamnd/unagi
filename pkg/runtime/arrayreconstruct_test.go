package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// call invokes the module-level _array_reconstructor the way an unpickler would,
// through its registered callable, so the test exercises the same entry point.
func callReconstructor(t *testing.T, pos ...objects.Object) (objects.Object, error) {
	t.Helper()
	return arrayReconstructor(pos, nil, nil)
}

func TestArrayReconstructorFastPath(t *testing.T) {
	if _, err := ImportModule("array"); err != nil {
		t.Fatalf("import array: %v", err)
	}
	// SIGNED_INT8 (mformat 1) is the native format for the 'b' type code, so the
	// bytes are read straight through: two signed bytes -1 and 2.
	got, err := callReconstructor(t, arrayType, objects.NewStr("b"), objects.NewInt(1), objects.NewBytes([]byte{0xff, 0x02}))
	if err != nil {
		t.Fatalf("reconstruct b: %v", err)
	}
	if r := objects.Repr(got); r != "array('b', [-1, 2])" {
		t.Fatalf("fast path = %s, want array('b', [-1, 2])", r)
	}
}

func TestArrayReconstructorSlowRetype(t *testing.T) {
	if _, err := ImportModule("array"); err != nil {
		t.Fatalf("import array: %v", err)
	}
	// UNSIGNED_INT64_BE (mformat 11) is not this machine's native 'L' format, so
	// the slow path decodes big-endian and retypes to 'Q', the code whose width and
	// signedness match an 8-byte unsigned value.
	be := []byte{0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 2}
	got, err := callReconstructor(t, arrayType, objects.NewStr("L"), objects.NewInt(11), objects.NewBytes(be))
	if err != nil {
		t.Fatalf("reconstruct L be: %v", err)
	}
	if r := objects.Repr(got); r != "array('Q', [1, 2])" {
		t.Fatalf("slow retype = %s, want array('Q', [1, 2])", r)
	}
}

func TestArrayReconstructorErrors(t *testing.T) {
	if _, err := ImportModule("array"); err != nil {
		t.Fatalf("import array: %v", err)
	}
	// A non-type first argument is a plain "not a type object" TypeError.
	if _, err := callReconstructor(t, objects.NewStr(""), objects.NewStr("b"), objects.NewInt(0), objects.NewBytes(nil)); err == nil {
		t.Fatal("non-type first argument did not raise")
	}
	// A machine format code outside 0..21 is rejected.
	if _, err := callReconstructor(t, arrayType, objects.NewStr("b"), objects.NewInt(22), objects.NewBytes(nil)); err == nil {
		t.Fatal("out-of-range mformat did not raise")
	}
	// A byte count that is not a whole number of items on the slow path raises;
	// mformat 17 (DOUBLE_BE) is not native so it takes the slow path length check.
	if _, err := callReconstructor(t, arrayType, objects.NewStr("d"), objects.NewInt(17), objects.NewBytes([]byte("a"))); err == nil {
		t.Fatal("short item bytes did not raise")
	}
	// The fourth argument must be bytes.
	if _, err := callReconstructor(t, arrayType, objects.NewStr("b"), objects.NewInt(0), objects.NewStr("")); err == nil {
		t.Fatal("non-bytes items did not raise")
	}
}
