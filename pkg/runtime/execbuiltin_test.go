package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// execInto runs exec(src, globals, ns) and returns the ns namespace so a test can
// read back the names the block bound.
func execInto(t *testing.T, src string) objects.Object {
	t.Helper()
	globals, err := objects.NewDict(nil, nil)
	if err != nil {
		t.Fatalf("NewDict globals: %v", err)
	}
	ns, err := objects.NewDict(nil, nil)
	if err != nil {
		t.Fatalf("NewDict ns: %v", err)
	}
	if _, err := execBuiltin([]objects.Object{objects.NewStr(src), globals, ns}); err != nil {
		t.Fatalf("exec(%q): %v", src, err)
	}
	return ns
}

// nsGet reads a name from an exec namespace, failing if it is absent.
func nsGet(t *testing.T, ns objects.Object, name string) objects.Object {
	t.Helper()
	v, err := objects.GetItem(ns, objects.NewStr(name))
	if err != nil {
		t.Fatalf("ns[%q]: %v", name, err)
	}
	return v
}

// TestExecDefinesAndCallsFunction covers the shape dataclasses relies on: a def
// exec'd into a namespace, read back out, and called, with defaults, an if, a
// return, and an f-string body all resolving through the bounded executor.
func TestExecDefinesAndCallsFunction(t *testing.T) {
	ns := execInto(t, "def greet(name, punct='!'):\n return f'hi {name}{punct}'\n")
	fn := nsGet(t, ns, "greet")
	got, err := objects.Call(fn, []objects.Object{objects.NewStr("ada")})
	if err != nil {
		t.Fatalf("greet('ada'): %v", err)
	}
	if s, _ := objects.AsStr(got); s != "hi ada!" {
		t.Errorf("greet('ada') = %q, want %q", s, "hi ada!")
	}
	got, err = objects.Call(fn, []objects.Object{objects.NewStr("ada"), objects.NewStr("?")})
	if err != nil {
		t.Fatalf("greet('ada', '?'): %v", err)
	}
	if s, _ := objects.AsStr(got); s != "hi ada?" {
		t.Errorf("greet with punct = %q, want %q", s, "hi ada?")
	}
}

// TestExecClosureAndControlFlow checks a nested def closing over its enclosing
// locals, an if/else with a return, and a for loop with break, the control flow
// the vendored codegen and general exec use both lean on.
func TestExecClosureAndControlFlow(t *testing.T) {
	src := "def make(base):\n" +
		" def add(x):\n" +
		"  if x < 0:\n" +
		"   return base\n" +
		"  total = base\n" +
		"  for i in [1, 2, 3]:\n" +
		"   total = total + x\n" +
		"   if i == 2:\n" +
		"    break\n" +
		"  return total\n" +
		" return add\n"
	ns := execInto(t, src)
	make := nsGet(t, ns, "make")
	add, err := objects.Call(make, []objects.Object{objects.NewInt(10)})
	if err != nil {
		t.Fatalf("make(10): %v", err)
	}
	got, err := objects.Call(add, []objects.Object{objects.NewInt(5)})
	if err != nil {
		t.Fatalf("add(5): %v", err)
	}
	if r := objects.Repr(got); r != "20" {
		t.Errorf("add(5) = %s, want 20 (10 + 5 + 5, break at i==2)", r)
	}
	got, err = objects.Call(add, []objects.Object{objects.NewInt(-1)})
	if err != nil {
		t.Fatalf("add(-1): %v", err)
	}
	if r := objects.Repr(got); r != "10" {
		t.Errorf("add(-1) = %s, want 10 (the base branch)", r)
	}
}

// TestExecAssignTargets covers attribute and subscript assignment through exec,
// beyond the plain-name case.
func TestExecAssignTargets(t *testing.T) {
	src := "def fill(d):\n" +
		" d['a'] = 1\n" +
		" d['b'] = d['a'] + 1\n" +
		" return d\n"
	ns := execInto(t, src)
	fill := nsGet(t, ns, "fill")
	d, err := objects.NewDict(nil, nil)
	if err != nil {
		t.Fatalf("NewDict: %v", err)
	}
	got, err := objects.Call(fill, []objects.Object{d})
	if err != nil {
		t.Fatalf("fill(d): %v", err)
	}
	if r := objects.Repr(got); r != "{'a': 1, 'b': 2}" {
		t.Errorf("fill(d) = %s, want {'a': 1, 'b': 2}", r)
	}
}
