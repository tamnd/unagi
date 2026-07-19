package objects

import "testing"

// TestNewMethodBindsSelf checks that a NewMethod placed in a NewClass dict binds
// the instance as self when read off an instance, the way a def-statement method
// does. A plain NewFunc would come back unbound.
func TestNewMethodBindsSelf(t *testing.T) {
	ident := NewMethod("ident", 1, func(a []Object) (Object, error) { return a[0], nil })
	cls, err := NewClass("C", "C", nil, []string{"ident"}, []Object{ident}, nil, nil)
	if err != nil {
		t.Fatalf("NewClass: %v", err)
	}
	inst, err := Call(cls, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	m, err := LoadAttr(inst, "ident")
	if err != nil {
		t.Fatalf("LoadAttr ident: %v", err)
	}
	r, err := Call(m, nil)
	if err != nil {
		t.Fatalf("call bound ident: %v", err)
	}
	if r != inst {
		t.Fatal("bound method should receive the instance as self")
	}
}

// TestWithBuiltinMethodManager drives the with protocol over a class whose
// __enter__/__exit__ are builtin methods rather than def-statement functions,
// the shape a Go-built classObject like _io._IOBase takes.
func TestWithBuiltinMethodManager(t *testing.T) {
	var events []string
	enter := NewMethod("__enter__", 1, func(a []Object) (Object, error) {
		events = append(events, "enter")
		return a[0], nil
	})
	exit := NewMethod("__exit__", -1, func(a []Object) (Object, error) {
		events = append(events, "exit")
		return None, nil
	})
	cls, err := NewClass("Mgr", "Mgr", nil,
		[]string{"__enter__", "__exit__"}, []Object{enter, exit}, nil, nil)
	if err != nil {
		t.Fatalf("NewClass: %v", err)
	}
	inst, err := Call(cls, nil)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	if !supportsSyncCM(inst) {
		t.Fatal("a class with builtin __enter__/__exit__ should support the with protocol")
	}
	exitFn, entered, err := WithEnter(inst)
	if err != nil {
		t.Fatalf("WithEnter: %v", err)
	}
	if entered != inst {
		t.Fatal("__enter__ should return the manager itself")
	}
	if _, err := Call(exitFn, []Object{None, None, None}); err != nil {
		t.Fatalf("exit: %v", err)
	}
	if len(events) != 2 || events[0] != "enter" || events[1] != "exit" {
		t.Fatalf("events = %v, want [enter exit]", events)
	}
}
