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

func TestUserExcCustomInit(t *testing.T) {
	// A user exception with a custom __init__ runs it against the exception
	// itself: super().__init__ resets args, self.x stores an attribute, and a
	// __str__ override drives str().
	exc, _ := ExcClass("Exception")
	// __init__(self, code, message): self.args = (message,); self.code = code.
	initFn := NewFunction("App.__init__",
		[]Param{{Name: "self", Kind: ParamPlain}, {Name: "code", Kind: ParamPlain}, {Name: "message", Kind: ParamPlain}},
		nil,
		func(a []Object) (Object, error) {
			self := a[0].(*Exception)
			self.Args = []Object{a[2]}
			if err := StoreAttr(self, "code", a[1]); err != nil {
				return nil, err
			}
			return None, nil
		})
	// __str__(self): return "[<code>]".
	strFn := NewFunction("App.__str__",
		[]Param{{Name: "self", Kind: ParamPlain}}, nil,
		func(a []Object) (Object, error) {
			self := a[0].(*Exception)
			code, err := LoadAttr(self, "code")
			if err != nil {
				return nil, err
			}
			return NewStr("[" + Str(code) + "]"), nil
		})
	app, err := newClassCore(nil, "App", "App", []Object{exc},
		[]string{"__init__", "__str__"}, []Object{initFn, strFn}, nil, nil)
	if err != nil {
		t.Fatalf("build App: %v", err)
	}
	ac := app.(*classObject)
	inst, err := Instantiate(ac, []Object{NewInt(404), NewStr("nf")}, nil, nil)
	if err != nil {
		t.Fatalf("Instantiate(App): %v", err)
	}
	e := inst.(*Exception)
	// args came from super().__init__(message), not the raw constructor pair.
	if len(e.Args) != 1 || Str(e.Args[0]) != "nf" {
		t.Errorf("args = %v, want (nf,)", e.Args)
	}
	// The stored attribute reads back, and str() runs the override.
	if got, err := LoadAttr(e, "code"); err != nil || Str(got) != "404" {
		t.Errorf("e.code = %v, %v", got, err)
	}
	if Str(e) != "[404]" {
		t.Errorf("str(e) = %q, want [404]", Str(e))
	}
	// repr keeps the default ClassName(args...) shape since no __repr__ override.
	if Repr(e) != "App('nf')" {
		t.Errorf("repr(e) = %q, want App('nf')", Repr(e))
	}
	// __dict__ reports the one stored attribute in insertion order.
	d, err := InstanceDict(e)
	if err != nil {
		t.Fatalf("vars(e): %v", err)
	}
	if Repr(d) != "{'code': 404}" {
		t.Errorf("__dict__ = %s, want {'code': 404}", Repr(d))
	}
}

func classNames(cs []*classObject) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.name
	}
	return out
}
