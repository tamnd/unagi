package objects

import "testing"

// PEP 562: a module-level __getattr__ is consulted for a name missing from the
// module namespace, and an AttributeError it raises surfaces unchanged.
func TestModuleGetAttrHook(t *testing.T) {
	m := NewModule("mymod", "mymod.py")
	hook := NewFunc("__getattr__", 1, func(args []Object) (Object, error) {
		name, _ := AsStr(args[0])
		if name == "MAGIC" {
			return NewInt(42), nil
		}
		return nil, Raise(AttributeError, "module 'mymod' has no attribute '%s'", name)
	})
	m.SetGlobal("__getattr__", hook)

	got, err := LoadAttr(m, "MAGIC")
	if err != nil {
		t.Fatalf("LoadAttr MAGIC: %v", err)
	}
	if n, ok := AsInt(got); !ok || n != 42 {
		t.Errorf("MAGIC = %v, want 42", got)
	}

	if _, err := LoadAttr(m, "MISSING"); err == nil {
		t.Error("expected AttributeError for a name the hook declines")
	}
}

// A bound name always wins over the hook, and __getattr__ itself is never
// resolved through the hook.
func TestModuleGetAttrHookNotForBoundName(t *testing.T) {
	m := NewModule("mymod", "mymod.py")
	called := false
	hook := NewFunc("__getattr__", 1, func(args []Object) (Object, error) {
		called = true
		return NewInt(0), nil
	})
	m.SetGlobal("__getattr__", hook)
	m.SetGlobal("bound", NewInt(7))

	if _, err := LoadAttr(m, "bound"); err != nil {
		t.Fatalf("LoadAttr bound: %v", err)
	}
	if called {
		t.Error("hook was consulted for a bound name")
	}
}

// A module-qualified builtin type reports its module as the part before the last
// dot, the read typing._alias makes off re.Match when it builds the Match alias.
func TestTypeObjectModule(t *testing.T) {
	qualified := TypeSingleton("re.Match")
	got, err := LoadAttr(qualified, "__module__")
	if err != nil {
		t.Fatalf("LoadAttr __module__: %v", err)
	}
	if s, _ := AsStr(got); s != "re" {
		t.Errorf("re.Match.__module__ = %q, want %q", s, "re")
	}

	bare := TypeSingleton("function")
	got, err = LoadAttr(bare, "__module__")
	if err != nil {
		t.Fatalf("LoadAttr __module__ bare: %v", err)
	}
	if s, _ := AsStr(got); s != "builtins" {
		t.Errorf("function.__module__ = %q, want %q", s, "builtins")
	}
}
