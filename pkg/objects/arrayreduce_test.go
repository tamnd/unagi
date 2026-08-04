package objects

import "testing"

// withArrayReduceEnv seeds the two hooks array.__reduce_ex__ reads (the type
// resolver and the reconstructor callable) that the runtime fills at import, so a
// pure object-model test can drive the reduction, and restores them after.
func withArrayReduceEnv(t *testing.T, fn func(arrayType, reconstructor Object)) {
	t.Helper()
	arrayType := NewFunc("array.array", -1, func(args []Object) (Object, error) { return None, nil })
	reconstructor := NewFunc("_array_reconstructor", -1, func(args []Object) (Object, error) { return None, nil })
	savedResolver, savedRecon := BuiltinTypeResolver, ArrayReconstructor
	BuiltinTypeResolver = func(name string) (Object, bool) {
		if name == "array.array" {
			return arrayType, true
		}
		return nil, false
	}
	ArrayReconstructor = reconstructor
	defer func() { BuiltinTypeResolver, ArrayReconstructor = savedResolver, savedRecon }()
	fn(arrayType, reconstructor)
}

func reduceExTuple(t *testing.T, a Object, protocol int64) *tupleObject {
	t.Helper()
	r, err := CallMethod(a, "__reduce_ex__", []Object{NewInt(protocol)})
	if err != nil {
		t.Fatalf("__reduce_ex__(%d): %v", protocol, err)
	}
	tup, ok := r.(*tupleObject)
	if !ok || len(tup.elts) != 3 {
		t.Fatalf("reduction shape = %s", Repr(r))
	}
	return tup
}

func TestArrayReduceExLegacy(t *testing.T) {
	withArrayReduceEnv(t, func(arrayType, _ Object) {
		// Below protocol 3 the reduction is (array type, (typecode, list), None).
		a, err := NewArray(NewStr("i"), NewList([]Object{NewInt(1), NewInt(2), NewInt(3)}))
		if err != nil {
			t.Fatalf("NewArray: %v", err)
		}
		tup := reduceExTuple(t, a, 2)
		if tup.elts[0] != arrayType {
			t.Fatalf("reduction callable = %s, want the array type", Repr(tup.elts[0]))
		}
		if r := Repr(tup.elts[1]); r != "('i', [1, 2, 3])" {
			t.Fatalf("reduction args = %s", r)
		}
		if tup.elts[2] != None {
			t.Fatalf("reduction state = %s, want None", Repr(tup.elts[2]))
		}
	})
}

func TestArrayReduceExMachineFormat(t *testing.T) {
	withArrayReduceEnv(t, func(arrayType, reconstructor Object) {
		a, err := NewArray(NewStr("i"), NewList([]Object{NewInt(1), NewInt(2)}))
		if err != nil {
			t.Fatalf("NewArray: %v", err)
		}
		tup := reduceExTuple(t, a, 5)
		if tup.elts[0] != reconstructor {
			t.Fatalf("reduction callable = %s, want the reconstructor", Repr(tup.elts[0]))
		}
		args, ok := tup.elts[1].(*tupleObject)
		if !ok || len(args.elts) != 4 {
			t.Fatalf("reduction args shape = %s", Repr(tup.elts[1]))
		}
		if args.elts[0] != arrayType {
			t.Fatalf("first arg = %s, want the array type", Repr(args.elts[0]))
		}
		// typecode 'i', machine format 8 (SIGNED_INT32_LE), little-endian bytes.
		if r := Repr(NewTuple(args.elts[1:])); r != "('i', 8, b'\\x01\\x00\\x00\\x00\\x02\\x00\\x00\\x00')" {
			t.Fatalf("machine-format args = %s", r)
		}
		if tup.elts[2] != None {
			t.Fatalf("reduction state = %s, want None", Repr(tup.elts[2]))
		}
	})
}

func TestArrayReduceExErrors(t *testing.T) {
	withArrayReduceEnv(t, func(_, _ Object) {
		a, err := NewArray(NewStr("b"), NewList(nil))
		if err != nil {
			t.Fatalf("NewArray: %v", err)
		}
		// No argument, too many arguments and a non-integer protocol each raise.
		if _, err := CallMethod(a, "__reduce_ex__", nil); err == nil {
			t.Fatal("__reduce_ex__() with no argument did not raise")
		}
		if _, err := CallMethod(a, "__reduce_ex__", []Object{NewInt(1), NewInt(2)}); err == nil {
			t.Fatal("__reduce_ex__() with two arguments did not raise")
		}
		if _, err := CallMethod(a, "__reduce_ex__", []Object{NewStr("x")}); err == nil {
			t.Fatal("__reduce_ex__() with a non-integer did not raise")
		}
	})
}
