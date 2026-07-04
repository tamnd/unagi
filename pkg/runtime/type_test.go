package runtime

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TypeOf hands back the constructor singleton for a builtin value, so the
// result is pointer-identical to the builtin name and reprs as a class.
func TestTypeOfBuiltinIdentity(t *testing.T) {
	cases := []struct {
		name string
		val  objects.Object
	}{
		{"int", objects.NewInt(5)},
		{"str", objects.NewStr("x")},
		{"float", objects.NewFloat(1.5)},
		{"bool", objects.True},
		{"list", objects.NewList(nil)},
		{"tuple", objects.NewTuple(nil)},
	}
	for _, c := range cases {
		got := TypeOf(c.val)
		if got != BuiltinFn(c.name) {
			t.Errorf("TypeOf(%s) not identical to the %s constructor", c.name, c.name)
		}
		if r := objects.Repr(got); r != "<class '"+c.name+"'>" {
			t.Errorf("repr(TypeOf(%s)) = %s, want <class '%s'>", c.name, r, c.name)
		}
	}
}

// A type value's own type is the type metatype, which is its own type in turn.
func TestTypeOfMetatype(t *testing.T) {
	meta := BuiltinFn("type")
	if TypeOf(BuiltinFn("int")) != meta {
		t.Error("type(int) is not the type metatype")
	}
	if TypeOf(meta) != meta {
		t.Error("type(type) is not type")
	}
}

// A constructor-less kind reports a cached type singleton, stable across calls.
func TestTypeOfSingletonStable(t *testing.T) {
	a := TypeOf(objects.None)
	b := TypeOf(objects.None)
	if a != b {
		t.Error("type(None) returned two different objects")
	}
	if r := objects.Repr(a); r != "<class 'NoneType'>" {
		t.Errorf("repr(type(None)) = %s, want <class 'NoneType'>", r)
	}
}

// A plain builtin function reports the builtin-function type, not a class.
func TestTypeOfBuiltinFunction(t *testing.T) {
	got := TypeOf(BuiltinFn("len"))
	if r := objects.Repr(got); r != "<class 'builtin_function_or_method'>" {
		t.Errorf("repr(type(len)) = %s, want <class 'builtin_function_or_method'>", r)
	}
}

// TypeCall enforces the 1-or-3 argument rule with the probed wording.
func TestTypeCallArity(t *testing.T) {
	for _, n := range []int{0, 2, 4} {
		args := make([]objects.Object, n)
		for i := range args {
			args[i] = objects.NewInt(1)
		}
		_, err := TypeCall(args)
		if err == nil || !strings.Contains(err.Error(), "type() takes 1 or 3 arguments") {
			t.Errorf("TypeCall(%d args) error = %v, want arity TypeError", n, err)
		}
	}
	_, err := TypeCall([]objects.Object{objects.NewInt(1), objects.NewInt(2), objects.NewInt(3)})
	if err == nil {
		t.Error("TypeCall(3 args) should raise the not-supported error")
	}
}
