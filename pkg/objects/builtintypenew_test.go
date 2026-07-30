package objects

import "testing"

// TestBuiltinTypeNew checks __new__ read off a builtin type object dispatches by
// the tp_new safety rules: T.__new__(T, ...) calls the type constructor, a
// subtype with its own __new__ is "not safe", and a non-subtype is rejected. The
// constructors are name-tagged stubs so the dispatch is observable here; the real
// int/bool/str values are covered end-to-end by the 2259 conformance fixture.
func TestBuiltinTypeNew(t *testing.T) {
	newOf := func(typeName string) Object {
		fn, ok := builtinTypeDunders[typeName]["__new__"]
		if !ok {
			t.Fatalf("%s.__new__ not registered", typeName)
		}
		return fn
	}
	// A stub standing in for a builtin type constructor: it carries the type name
	// the dispatch keys on and records the arguments it is called with.
	typeStub := func(name string, gotArgs *[]Object) Object {
		return NewFunc(name, -1, func(args []Object) (Object, error) {
			*gotArgs = args
			return NewStr("built:" + name), nil
		})
	}

	// int.__new__(int, 5): same type, so the constructor runs on the remaining
	// arguments.
	var got []Object
	r, err := Call(newOf("int"), []Object{typeStub("int", &got), NewInt(5)})
	if err != nil {
		t.Fatalf("int.__new__(int, 5): %v", err)
	}
	if s, _ := AsStr(r); s != "built:int" {
		t.Errorf("int.__new__(int, 5) = %s, want the int constructor result", Repr(r))
	}
	if len(got) != 1 || Repr(got[0]) != "5" {
		t.Errorf("int constructor got %v, want [5]", got)
	}

	// bool.__new__(bool): same type, constructor runs with no extra arguments.
	got = nil
	if _, err = Call(newOf("bool"), []Object{typeStub("bool", &got)}); err != nil {
		t.Fatalf("bool.__new__(bool): %v", err)
	}
	if len(got) != 0 {
		t.Errorf("bool constructor got %v, want []", got)
	}

	// The safety matrix: a subtype with its own __new__ is not safe, a non-subtype
	// is not a subtype.
	for _, tc := range []struct {
		self, arg, wantMsg string
	}{
		{"int", "bool", "int.__new__(bool) is not safe, use bool.__new__()"},
		{"bool", "int", "bool.__new__(int): int is not a subtype of bool"},
		{"int", "str", "int.__new__(str): str is not a subtype of int"},
	} {
		var unused []Object
		_, err := Call(newOf(tc.self), []Object{typeStub(tc.arg, &unused), NewInt(0)})
		exc, ok := err.(*Exception)
		if !ok || exc.Kind != TypeError {
			t.Errorf("%s.__new__(%s): err = %v, want TypeError", tc.self, tc.arg, err)
			continue
		}
		if got := exc.Text(); got != tc.wantMsg {
			t.Errorf("%s.__new__(%s) message = %q, want %q", tc.self, tc.arg, got, tc.wantMsg)
		}
		if unused != nil {
			t.Errorf("%s.__new__(%s) called the constructor, want it rejected first", tc.self, tc.arg)
		}
	}
}
