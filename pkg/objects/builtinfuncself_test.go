package objects

import "testing"

// TestBuiltinFuncSelfResolvesModule checks a plain builtin function reads
// __self__ through the runtime resolver, the way CPython binds a
// builtin_function_or_method to the module it lives in. A function the resolver
// declines keeps its missing-attribute report. The builtins-module identity half
// (len.__self__ is builtins) needs the runtime and is covered by the fixture.
func TestBuiltinFuncSelfResolvesModule(t *testing.T) {
	saved := BuiltinFuncSelf
	defer func() { BuiltinFuncSelf = saved }()

	lenFn := NewFunc("len", 1, func(args []Object) (Object, error) { return None, nil })
	sentinel := NewStr("<builtins-module-stub>")
	BuiltinFuncSelf = func(fn Object) (Object, bool) {
		if fn == lenFn {
			return sentinel, true
		}
		return nil, false
	}

	self, err := LoadAttr(lenFn, "__self__")
	if err != nil {
		t.Fatalf("len.__self__: %v", err)
	}
	if self != sentinel {
		t.Fatalf("len.__self__ = %v; want the resolver's module", self)
	}

	// A function the resolver declines keeps the AttributeError, so a native-module
	// function reaching the same path is not handed the builtins module.
	other := NewFunc("otherfn", 1, func(args []Object) (Object, error) { return None, nil })
	if _, err := LoadAttr(other, "__self__"); err == nil {
		t.Fatalf("otherfn.__self__ = no error; want AttributeError")
	}
}
