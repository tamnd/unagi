package objects

import "testing"

// echoFn builds a function object whose impl returns the bound argument
// tuple, so tests can see exactly what the binder produced.
func echoFn(qual string, params []Param, defaults []Object) Object {
	return NewFunction(qual, params, defaults, func(args []Object) (Object, error) {
		return NewTuple(args), nil
	})
}

func TestBindPositionalAndKeywords(t *testing.T) {
	// def f(a, b=2, /, c=3, *rest, k, kk=5, **kw), all wordings probed.
	params := []Param{
		{Name: "a", Kind: ParamPosOnly},
		{Name: "b", Kind: ParamPosOnly},
		{Name: "c", Kind: ParamPlain},
		{Name: "rest", Kind: ParamStar},
		{Name: "k", Kind: ParamKwOnly},
		{Name: "kk", Kind: ParamKwOnly},
		{Name: "kw", Kind: ParamStarStar},
	}
	defaults := []Object{nil, NewInt(2), NewInt(3), nil, nil, NewInt(5), nil}
	f := echoFn("f", params, defaults)

	got, err := CallKw(f, []Object{NewInt(1)}, []string{"k"}, []Object{NewInt(4)})
	if err != nil {
		t.Fatalf("minimal call: %v", err)
	}
	if s := Repr(got); s != "(1, 2, 3, (), 4, 5, {})" {
		t.Errorf("minimal call bound %s", s)
	}

	got, err = CallKw(f,
		[]Object{NewInt(1), NewInt(2), NewInt(3), NewInt(9)},
		[]string{"k", "z"}, []Object{NewInt(4), NewInt(7)})
	if err != nil {
		t.Fatalf("full call: %v", err)
	}
	if s := Repr(got); s != "(1, 2, 3, (9,), 4, 5, {'z': 7})" {
		t.Errorf("full call bound %s", s)
	}
}

func TestBindErrors(t *testing.T) {
	simple := echoFn("f", []Param{
		{Name: "a", Kind: ParamPosOnly},
		{Name: "count", Kind: ParamPlain},
	}, []Object{nil, NewInt(1)})
	kwonly := echoFn("g", []Param{
		{Name: "x", Kind: ParamPlain},
		{Name: "k", Kind: ParamKwOnly},
		{Name: "kk", Kind: ParamKwOnly},
	}, nil)

	tests := []struct {
		name    string
		f       Object
		pos     []Object
		kwNames []string
		kwVals  []Object
		want    string
	}{
		{"missing positional", simple, nil, nil, nil,
			"TypeError: f() missing 1 required positional argument: 'a'"},
		{"too many with defaults", simple, []Object{NewInt(1), NewInt(2), NewInt(3)}, nil, nil,
			"TypeError: f() takes from 1 to 2 positional arguments but 3 were given"},
		{"posonly as keyword", simple, []Object{NewInt(1)}, []string{"a"}, []Object{NewInt(2)},
			"TypeError: f() got some positional-only arguments passed as keyword arguments: 'a'"},
		{"unexpected with suggestion", simple, []Object{NewInt(1)}, []string{"cout"}, []Object{NewInt(2)},
			"TypeError: f() got an unexpected keyword argument 'cout'. Did you mean 'count'?"},
		{"multiple values", simple, []Object{NewInt(1), NewInt(2)}, []string{"count"}, []Object{NewInt(3)},
			"TypeError: f() got multiple values for argument 'count'"},
		{"missing kwonly pair", kwonly, []Object{NewInt(1)}, nil, nil,
			"TypeError: g() missing 2 required keyword-only arguments: 'k' and 'kk'"},
		{"too many exact", kwonly, []Object{NewInt(1), NewInt(2)}, []string{"k", "kk"}, []Object{NewInt(3), NewInt(4)},
			"TypeError: g() takes 1 positional argument but 2 positional arguments (and 2 keyword-only arguments) were given"},
	}
	for _, tt := range tests {
		_, err := CallKw(tt.f, tt.pos, tt.kwNames, tt.kwVals)
		checkErr(t, tt.name, err, tt.want)
	}
}

func TestCallRoutesFunctionObject(t *testing.T) {
	f := echoFn("f", []Param{{Name: "a", Kind: ParamPlain}}, nil)
	got, err := Call(f, []Object{NewInt(7)})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if s := Repr(got); s != "(7,)" {
		t.Errorf("Call bound %s", s)
	}
	_, err = Call(f, []Object{NewInt(1), NewInt(2)})
	checkErr(t, "Call too many", err,
		"TypeError: f() takes 1 positional argument but 2 were given")
	_, err = Call(NewInt(3), nil)
	checkErr(t, "int not callable", err, "TypeError: 'int' object is not callable")
	_, err = CallKw(NewStr("s"), nil, []string{"k"}, []Object{None})
	checkErr(t, "str not callable kw", err, "TypeError: 'str' object is not callable")
}

func TestFuncObjectRejectsKeywords(t *testing.T) {
	f := NewFunc("len", 1, func(args []Object) (Object, error) { return None, nil })
	_, err := CallKw(f, []Object{NewInt(1)}, []string{"x"}, []Object{NewInt(2)})
	checkErr(t, "builtin kw", err, "TypeError: len() takes no keyword arguments")
	if _, err := CallKw(f, []Object{NewInt(1)}, nil, nil); err != nil {
		t.Errorf("builtin positional through CallKw: %v", err)
	}
}

func TestQualifyBuiltin(t *testing.T) {
	f := NewFunc("comb", 2, func(args []Object) (Object, error) { return None, nil })
	QualifyBuiltin(f, "math.comb")
	_, err := CallKw(f, []Object{NewInt(1), NewInt(2)}, []string{"x"}, []Object{NewInt(3)})
	checkErr(t, "qualified kw", err, "TypeError: math.comb() takes no keyword arguments")
	// It is a no-op for anything that is not a plain builtin function.
	QualifyBuiltin(NewInt(1), "math.x")
}

// TestFunctionCodeFirstLine checks __code__.co_firstlineno reports the def line
// a def emit site recorded through WithFuncFirstLine, and stays 0 for a function
// built with no line (the native module functions). Probed on 3.14: co_firstlineno
// is the line the def header sits on and is read-only.
func TestFunctionCodeFirstLine(t *testing.T) {
	firstLine := func(fn Object) int64 {
		code, err := LoadAttr(fn, "__code__")
		if err != nil {
			t.Fatalf("__code__: %v", err)
		}
		v, err := LoadAttr(code, "co_firstlineno")
		if err != nil {
			t.Fatalf("co_firstlineno: %v", err)
		}
		n, ok := AsInt(v)
		if !ok {
			t.Fatalf("co_firstlineno is not an int: %v", v)
		}
		return n
	}

	f := echoFn("f", nil, nil)
	if got := firstLine(f); got != 0 {
		t.Errorf("co_firstlineno with no recorded line = %d, want 0", got)
	}

	WithFuncFirstLine(f, 42)
	if got := firstLine(f); got != 42 {
		t.Errorf("co_firstlineno after WithFuncFirstLine(42) = %d, want 42", got)
	}
}

func TestFunctionIdentitySemantics(t *testing.T) {
	f := echoFn("f", nil, nil)
	g := echoFn("g", nil, nil)
	if !Truth(f) {
		t.Error("functions must be truthy")
	}
	eq, err := Compare(OpEq, f, f)
	if err != nil || !Truth(eq) {
		t.Errorf("f == f gave %v, %v", eq, err)
	}
	eq, err = Compare(OpEq, f, g)
	if err != nil || Truth(eq) {
		t.Errorf("f == g gave %v, %v", eq, err)
	}
	h1, err := PyHash(f)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	h2, err := PyHash(f)
	if err != nil || h1 != h2 {
		t.Errorf("hash not stable: %d %d %v", h1, h2, err)
	}
	d, err := NewDict([]Object{f, g}, []Object{NewInt(1), NewInt(2)})
	if err != nil {
		t.Fatalf("functions as dict keys: %v", err)
	}
	v, err := GetItem(d, f)
	if err != nil {
		t.Fatalf("dict lookup by function: %v", err)
	}
	if n, _ := AsInt(v); n != 1 {
		t.Errorf("dict lookup got %v", v)
	}
}
