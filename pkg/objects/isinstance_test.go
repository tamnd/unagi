package objects

import (
	"strings"
	"testing"
)

// bt is the builtin type value for name, the funcObject the runtime hands to a
// type argument. Tests use TypeSingleton for constructor-less kinds directly.
func bt(name string) Object { return NewFunc(name, -1, nil) }

func wantIsBool(t *testing.T, got Object, err error, want bool, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: unexpected error %v", msg, err)
	}
	if (got == True) != want {
		t.Errorf("%s = %v, want %v", msg, got == True, want)
	}
}

// isinstance over builtins follows the subtype lattice: int owns bool, the
// kinds are otherwise flat, and type matches every type value.
func TestIsInstanceBuiltin(t *testing.T) {
	r, err := IsInstance(NewInt(5), bt("int"))
	wantIsBool(t, r, err, true, "isinstance(5, int)")
	r, err = IsInstance(True, bt("int"))
	wantIsBool(t, r, err, true, "isinstance(True, int)")
	r, err = IsInstance(True, bt("bool"))
	wantIsBool(t, r, err, true, "isinstance(True, bool)")
	r, err = IsInstance(NewInt(5), bt("bool"))
	wantIsBool(t, r, err, false, "isinstance(5, bool)")
	r, err = IsInstance(NewFloat(1), bt("int"))
	wantIsBool(t, r, err, false, "isinstance(1.0, int)")
	r, err = IsInstance(NewStr("x"), bt("str"))
	wantIsBool(t, r, err, true, "isinstance('x', str)")
}

// A builtin type value, and only a type value, is an instance of type.
func TestIsInstanceType(t *testing.T) {
	r, err := IsInstance(bt("int"), bt("type"))
	wantIsBool(t, r, err, true, "isinstance(int, type)")
	r, err = IsInstance(bt("type"), bt("type"))
	wantIsBool(t, r, err, true, "isinstance(type, type)")
	r, err = IsInstance(NewInt(5), bt("type"))
	wantIsBool(t, r, err, false, "isinstance(5, type)")
	r, err = IsInstance(TypeSingleton("NoneType"), bt("type"))
	wantIsBool(t, r, err, true, "isinstance(type(None), type)")
}

// A tuple of types matches when any element matches, builtins included.
func TestIsInstanceTuple(t *testing.T) {
	types := NewTuple([]Object{bt("str"), bt("float"), bt("int")})
	r, err := IsInstance(NewInt(5), types)
	wantIsBool(t, r, err, true, "isinstance(5, (str, float, int))")
	miss := NewTuple([]Object{bt("str"), bt("float")})
	r, err = IsInstance(NewInt(5), miss)
	wantIsBool(t, r, err, false, "isinstance(5, (str, float))")
}

// issubclass over builtins: reflexive, bool descends from int, kinds otherwise
// flat, and a builtin type is never a subclass of a user class.
func TestIsSubclassBuiltin(t *testing.T) {
	r, err := IsSubclass(bt("bool"), bt("int"))
	wantIsBool(t, r, err, true, "issubclass(bool, int)")
	r, err = IsSubclass(bt("int"), bt("int"))
	wantIsBool(t, r, err, true, "issubclass(int, int)")
	r, err = IsSubclass(bt("int"), bt("bool"))
	wantIsBool(t, r, err, false, "issubclass(int, bool)")
	r, err = IsSubclass(bt("int"), bt("float"))
	wantIsBool(t, r, err, false, "issubclass(int, float)")
	r, err = IsSubclass(bt("int"), bt("type"))
	wantIsBool(t, r, err, false, "issubclass(int, type)")
	r, err = IsSubclass(bt("type"), bt("type"))
	wantIsBool(t, r, err, true, "issubclass(type, type)")
	r, err = IsSubclass(bt("bool"), NewTuple([]Object{bt("str"), bt("int")}))
	wantIsBool(t, r, err, true, "issubclass(bool, (str, int))")
}

// A non-class first argument raises the arg 1 TypeError before arg 2 is judged.
func TestIsSubclassArg1(t *testing.T) {
	_, err := IsSubclass(NewInt(5), bt("int"))
	if err == nil || !strings.Contains(err.Error(), "issubclass() arg 1 must be a class") {
		t.Errorf("issubclass(5, int) error = %v, want arg 1 TypeError", err)
	}
}

// A non-type second argument raises the probed isinstance arg 2 TypeError.
func TestIsInstanceArg2(t *testing.T) {
	_, err := IsInstance(NewInt(5), NewInt(3))
	if err == nil || !strings.Contains(err.Error(), "isinstance() arg 2 must be a type, a tuple of types, or a union") {
		t.Errorf("isinstance(5, 3) error = %v, want arg 2 TypeError", err)
	}
}

// A self-match builtin binds the subject through one positional slot; a
// non-self-match builtin rejects any positional with the probed wording.
func TestMatchBuiltinClass(t *testing.T) {
	names, ok, err := MatchClass(NewInt(5), bt("int"), 1, nil)
	if err != nil || !ok || len(names) != 1 || names[0] != selfMatchSentinel {
		t.Fatalf("MatchClass(5, int, 1) = %v, %v, %v", names, ok, err)
	}
	subj := NewInt(5)
	bound, ok, err := MatchClassAttr(subj, names[0])
	if err != nil || !ok || bound != subj {
		t.Errorf("self-match slot did not bind the subject: %v %v %v", bound, ok, err)
	}
	if _, ok, _ := MatchClass(NewStr("x"), bt("int"), 0, nil); ok {
		t.Error("MatchClass('x', int) should not match")
	}
	_, _, err = MatchClass(NewInt(5), bt("int"), 2, nil)
	if err == nil || !strings.Contains(err.Error(), "int() accepts 1 positional sub-pattern (2 given)") {
		t.Errorf("MatchClass(5, int, 2) error = %v, want too-many TypeError", err)
	}
	_, _, err = MatchClass(NewRange(0, 3, 1), bt("range"), 1, nil)
	if err == nil || !strings.Contains(err.Error(), "range() accepts 0 positional sub-patterns (1 given)") {
		t.Errorf("MatchClass(range, 1) error = %v, want zero-positional TypeError", err)
	}
}
