package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// evalString is a test helper: eval(src) with no namespace.
func evalString(t *testing.T, src string) objects.Object {
	t.Helper()
	v, err := evalBuiltin([]objects.Object{objects.NewStr(src)})
	if err != nil {
		t.Fatalf("eval(%q) errored: %v", src, err)
	}
	return v
}

// evalRepr reprs the result of eval(src) for easy golden comparison.
func evalRepr(t *testing.T, src string) string {
	t.Helper()
	r, err := ReprOf(evalString(t, src))
	if err != nil {
		t.Fatalf("repr after eval(%q): %v", src, err)
	}
	s, ok := objects.AsStr(r)
	if !ok {
		t.Fatalf("repr of eval(%q) is not a str", src)
	}
	return s
}

func TestEvalLiteralsAndOperators(t *testing.T) {
	cases := []struct{ src, want string }{
		{"1 + 2 * 3", "7"},
		{"(1 + 2) * 3", "9"},
		{"7 // 2", "3"},
		{"7 / 2", "3.5"},
		{"2 ** 10", "1024"},
		{"10 % 3", "1"},
		{"-5", "-5"},
		{"~0", "-1"},
		{"not 0", "True"},
		{"1 < 2 <= 2 < 3", "True"},
		{"1 < 2 > 3", "False"},
		{"1 == 1 != 2", "True"},
		{"'a' + 'b'", "'ab'"},
		{"'ab' * 3", "'ababab'"},
		{"[1, 2, 3]", "[1, 2, 3]"},
		{"(1, 2, 3)", "(1, 2, 3)"},
		{"{'x': 1, 'y': 2}", "{'x': 1, 'y': 2}"},
		{"1 if True else 2", "1"},
		{"1 if False else 2", "2"},
		{"True and 5", "5"},
		{"0 or 'fallback'", "'fallback'"},
		{"2 in [1, 2, 3]", "True"},
		{"4 not in [1, 2, 3]", "True"},
		{"None is None", "True"},
		{"[1, 2, 3][1]", "2"},
		{"{'k': 9}['k']", "9"},
		{"'HELLO'.lower()", "'hello'"},
		{"len([1, 2, 3])", "3"},
		{"max(3, 7, 1)", "7"},
	}
	for _, c := range cases {
		if got := evalRepr(t, c.src); got != c.want {
			t.Errorf("eval(%q) = %s, want %s", c.src, got, c.want)
		}
	}
}

// TestEvalNameResolution checks the locals -> globals -> builtins chain.
func TestEvalNameResolution(t *testing.T) {
	globals, err := objects.NewDict(
		[]objects.Object{objects.NewStr("g")},
		[]objects.Object{objects.NewInt(10)},
	)
	if err != nil {
		t.Fatal(err)
	}
	locals, err := objects.NewDict(
		[]objects.Object{objects.NewStr("l")},
		[]objects.Object{objects.NewInt(5)},
	)
	if err != nil {
		t.Fatal(err)
	}
	v, err := evalBuiltin([]objects.Object{objects.NewStr("g + l"), globals, locals})
	if err != nil {
		t.Fatalf("eval with namespaces: %v", err)
	}
	if n, ok := objects.AsInt(v); !ok || n != 15 {
		t.Fatalf("g + l = %v, want 15", v)
	}
	// A name in neither namespace nor the builtins is a NameError.
	if _, err := evalBuiltin([]objects.Object{objects.NewStr("missing")}); err == nil {
		t.Fatal("eval of an undefined name should raise NameError")
	}
}

// TestEvalNamedtupleLambda reproduces exactly what collections.namedtuple does:
// it eval's a lambda whose parameter list is the field names, capturing a
// _tuple_new helper from the globals namespace, and calls the result. This is
// the case the whole builtin exists to support.
func TestEvalNamedtupleLambda(t *testing.T) {
	// A stand-in for tuple.__new__: builds a tuple from (cls, elements).
	tupleNew := objects.NewFunc("_tuple_new", 2, func(args []objects.Object) (objects.Object, error) {
		return args[1], nil
	})
	globals, err := objects.NewDict(
		[]objects.Object{objects.NewStr("_tuple_new")},
		[]objects.Object{tupleNew},
	)
	if err != nil {
		t.Fatal(err)
	}
	code := "lambda _cls, x, y: _tuple_new(_cls, (x, y))"
	fn, err := evalBuiltin([]objects.Object{objects.NewStr(code), globals})
	if err != nil {
		t.Fatalf("eval lambda: %v", err)
	}
	got, err := objects.Call(fn, []objects.Object{
		objects.NewStr("Point"), objects.NewInt(3), objects.NewInt(4),
	})
	if err != nil {
		t.Fatalf("call eval'd lambda: %v", err)
	}
	r, err := ReprOf(got)
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := objects.AsStr(r); s != "(3, 4)" {
		t.Fatalf("namedtuple lambda produced %s, want (3, 4)", s)
	}
}

// TestEvalDefaultsAndKeywords checks lambda defaults and keyword calls through
// the eval'd function.
func TestEvalDefaultsAndKeywords(t *testing.T) {
	fn, err := evalBuiltin([]objects.Object{objects.NewStr("lambda a, b=100: a + b")})
	if err != nil {
		t.Fatal(err)
	}
	got, err := objects.Call(fn, []objects.Object{objects.NewInt(1)})
	if err != nil {
		t.Fatal(err)
	}
	if n, ok := objects.AsInt(got); !ok || n != 101 {
		t.Fatalf("default not applied: got %v, want 101", got)
	}
	got, err = objects.CallKw(fn,
		[]objects.Object{objects.NewInt(1)},
		[]string{"b"}, []objects.Object{objects.NewInt(2)})
	if err != nil {
		t.Fatal(err)
	}
	if n, ok := objects.AsInt(got); !ok || n != 3 {
		t.Fatalf("keyword arg not honored: got %v, want 3", got)
	}
}

// TestEvalRejectsStatementsAndUnsupported confirms the bounded surface reports
// clear errors instead of mis-evaluating.
func TestEvalRejectsStatementsAndUnsupported(t *testing.T) {
	for _, src := range []string{
		"x = 1",          // a statement, not an expression
		"import os",      // a statement
		"[x for x in y]", // comprehension, out of scope
		"f(*args)",       // starred call argument, out of scope
		"a[1:2]",         // slice, out of scope
	} {
		if _, err := evalBuiltin([]objects.Object{objects.NewStr(src)}); err == nil {
			t.Errorf("eval(%q) should have errored", src)
		}
	}
}

// TestEvalArity checks the argument-count guard.
func TestEvalArity(t *testing.T) {
	if _, err := evalBuiltin(nil); err == nil {
		t.Error("eval() with no args should error")
	}
	if _, err := evalBuiltin([]objects.Object{
		objects.NewStr("1"), objects.None, objects.None, objects.None,
	}); err == nil {
		t.Error("eval() with 4 args should error")
	}
	// A non-string source is a TypeError.
	if _, err := evalBuiltin([]objects.Object{objects.NewInt(1)}); err == nil {
		t.Error("eval() of a non-string should error")
	}
}
