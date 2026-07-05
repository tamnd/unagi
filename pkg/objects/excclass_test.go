package objects

import "testing"

func TestExcClassHierarchy(t *testing.T) {
	// Every built-in exception name resolves to a class object whose MRO mirrors
	// the base table, so it can be subclassed and matched like a user class.
	ve, ok := ExcClass("ValueError")
	if !ok {
		t.Fatal("ExcClass(ValueError) missing")
	}
	if ve.name != "ValueError" || ve.qual != "ValueError" {
		t.Errorf("ValueError name/qual = %q/%q", ve.name, ve.qual)
	}
	exc, _ := ExcClass("Exception")
	base, _ := ExcClass("BaseException")
	// ValueError -> Exception -> BaseException, object stripped from the stored MRO.
	wantMRO := []*classObject{ve, exc, base}
	if len(ve.mro) != len(wantMRO) {
		t.Fatalf("ValueError MRO = %v, want %v", classNames(ve.mro), classNames(wantMRO))
	}
	for i, c := range wantMRO {
		if ve.mro[i] != c {
			t.Fatalf("ValueError MRO = %v, want %v", classNames(ve.mro), classNames(wantMRO))
		}
	}
}

func TestExcClassAliasIdentity(t *testing.T) {
	os, _ := ExcClass("OSError")
	io, _ := ExcClass("IOError")
	env, _ := ExcClass("EnvironmentError")
	if io != os || env != os {
		t.Errorf("OSError aliases are distinct: OSError=%p IOError=%p EnvironmentError=%p", os, io, env)
	}
}

func TestExcClassIsInstanceOfRaised(t *testing.T) {
	e := NewException("KeyError", []Object{NewStr("k")})
	lookup, _ := ExcClass("LookupError")
	typeErr, _ := ExcClass("TypeError")
	if r, err := IsInstance(e, lookup); err != nil || r != True {
		t.Errorf("isinstance(KeyError, LookupError) = %v, %v", r, err)
	}
	if r, err := IsInstance(e, typeErr); err != nil || r != False {
		t.Errorf("isinstance(KeyError, TypeError) = %v, %v", r, err)
	}
	// type(e) reports the exception's own class, so identity against it holds.
	if got, ok := ClassOf(e); !ok || got.(*classObject).name != "KeyError" {
		t.Errorf("ClassOf(KeyError) = %v, %v", got, ok)
	}
}

func classNames(cs []*classObject) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.name
	}
	return out
}
