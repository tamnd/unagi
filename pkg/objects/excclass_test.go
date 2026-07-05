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

func TestUserExcSubclassRaisable(t *testing.T) {
	// A user class subclassing a built-in exception builds an *Exception, not a
	// plain instance, so it is raisable and reports its own class identity.
	exc, _ := ExcClass("Exception")
	myErr, err := newClassCore(nil, "MyError", "MyError", []Object{exc}, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("build MyError: %v", err)
	}
	mc := myErr.(*classObject)
	inst, err := Instantiate(mc, []Object{NewStr("boom")}, nil, nil)
	if err != nil {
		t.Fatalf("Instantiate(MyError): %v", err)
	}
	e, ok := inst.(*Exception)
	if !ok {
		t.Fatalf("Instantiate(MyError) = %T, want *Exception", inst)
	}
	if e.Kind != "MyError" || e.Class != mc {
		t.Errorf("exception Kind/Class = %q/%p, want MyError/%p", e.Kind, e.Class, mc)
	}
	// type(e) reports MyError, and it is an instance of both MyError and its base.
	if got, ok := ClassOf(e); !ok || got != mc {
		t.Errorf("ClassOf = %v, %v", got, ok)
	}
	if r, err := IsInstance(e, mc); err != nil || r != True {
		t.Errorf("isinstance(e, MyError) = %v, %v", r, err)
	}
	if r, err := IsInstance(e, exc); err != nil || r != True {
		t.Errorf("isinstance(e, Exception) = %v, %v", r, err)
	}
	// A matcher matches on the class MRO: MyError and Exception catch it, an
	// unrelated exception class does not.
	typeErr, _ := ExcClass("TypeError")
	if !ExcMatchesClass(e, mc) || !ExcMatchesClass(e, exc) {
		t.Error("ExcMatchesClass(e, MyError/Exception) = false, want true")
	}
	if ExcMatchesClass(e, typeErr) {
		t.Error("ExcMatchesClass(e, TypeError) = true, want false")
	}
	// A bare class raises a no-argument instance of itself.
	bare, ok := AsRaisable(mc)
	if !ok || bare.Class != mc || len(bare.Args) != 0 {
		t.Errorf("AsRaisable(MyError) = %v, %v, args %v", bare, ok, bare.Args)
	}
}

func classNames(cs []*classObject) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.name
	}
	return out
}
