package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// The attribute builtins reuse the LoadAttr/StoreAttr/DelAttr machinery, so
// these tests pin the parts they add on top: the default-on-AttributeError of
// getattr, the True/False of hasattr, the non-string-name TypeError, and the
// arity wordings. Every message was probed against python3.14 (3.14.6).

func TestGetAttrDefault(t *testing.T) {
	e := objects.Raise(objects.ValueError, "x")

	got, err := GetAttr(objects.MainThread(), []objects.Object{e, objects.NewStr("args")})
	if err != nil {
		t.Fatalf("getattr args = %v", err)
	}
	if objects.Repr(got) != "('x',)" {
		t.Errorf("getattr args = %s", objects.Repr(got))
	}

	// A missing attribute with a default returns the default.
	d := objects.NewStr("fallback")
	got, err = GetAttr(objects.MainThread(), []objects.Object{e, objects.NewStr("nope"), d})
	if err != nil || got != d {
		t.Errorf("getattr default = %v, %v", got, err)
	}

	// Missing with no default propagates the AttributeError.
	_, err = GetAttr(objects.MainThread(), []objects.Object{e, objects.NewStr("nope")})
	checkErr(t, "getattr miss", err, "AttributeError: 'ValueError' object has no attribute 'nope'")
}

func TestHasAttr(t *testing.T) {
	e := objects.Raise(objects.ValueError, "x")
	got, err := HasAttr(objects.MainThread(), []objects.Object{e, objects.NewStr("args")})
	if err != nil || got != objects.True {
		t.Errorf("hasattr present = %v, %v", got, err)
	}
	got, err = HasAttr(objects.MainThread(), []objects.Object{e, objects.NewStr("nope")})
	if err != nil || got != objects.False {
		t.Errorf("hasattr absent = %v, %v", got, err)
	}
}

func TestSetAttrOnException(t *testing.T) {
	e := objects.Raise(objects.ValueError, "x")
	cause := objects.Raise(objects.KeyError, "k")
	got, err := SetAttr(objects.MainThread(), []objects.Object{e, objects.NewStr("__cause__"), cause})
	if err != nil || got != objects.None {
		t.Fatalf("setattr = %v, %v", got, err)
	}
	if e.Cause != cause {
		t.Errorf("setattr __cause__ did not take: %v", e.Cause)
	}
}

func TestAttrNameMustBeString(t *testing.T) {
	e := objects.Raise(objects.ValueError, "x")
	name := objects.NewInt(5)
	_, err := GetAttr(objects.MainThread(), []objects.Object{e, name})
	checkErr(t, "getattr name", err, "TypeError: attribute name must be string, not 'int'")
	_, err = HasAttr(objects.MainThread(), []objects.Object{e, name})
	checkErr(t, "hasattr name", err, "TypeError: attribute name must be string, not 'int'")
	_, err = SetAttr(objects.MainThread(), []objects.Object{e, name, objects.None})
	checkErr(t, "setattr name", err, "TypeError: attribute name must be string, not 'int'")
	_, err = DelAttr(objects.MainThread(), []objects.Object{e, name})
	checkErr(t, "delattr name", err, "TypeError: attribute name must be string, not 'int'")
}

func TestAttrArity(t *testing.T) {
	e := objects.Raise(objects.ValueError, "x")
	one := []objects.Object{e}
	_, err := GetAttr(objects.MainThread(), one)
	checkErr(t, "getattr arity", err, "TypeError: getattr expected at least 2 arguments, got 1")
	_, err = GetAttr(objects.MainThread(), []objects.Object{e, e, e, e})
	checkErr(t, "getattr arity max", err, "TypeError: getattr expected at most 3 arguments, got 4")
	_, err = HasAttr(objects.MainThread(), one)
	checkErr(t, "hasattr arity", err, "TypeError: hasattr expected 2 arguments, got 1")
	_, err = SetAttr(objects.MainThread(), []objects.Object{e, e})
	checkErr(t, "setattr arity", err, "TypeError: setattr expected 3 arguments, got 2")
	_, err = DelAttr(objects.MainThread(), one)
	checkErr(t, "delattr arity", err, "TypeError: delattr expected 2 arguments, got 1")
}
