package objects

import "testing"

// TestBuiltinFunctionCallAttr checks that a builtin function exposes a __call__
// that forwards to it, the method-wrapper CPython hangs on a
// builtin_function_or_method. This is what callable-introspection and
// unittest's assertHasAttr(func, '__call__') read; before it, getattr(len,
// '__call__') raised AttributeError.
func TestBuiltinFunctionCallAttr(t *testing.T) {
	// A one-argument builtin that echoes its argument, so a call through the
	// wrapper reads back what it was handed.
	fn := NewFunc("echo", 1, func(args []Object) (Object, error) { return args[0], nil })

	call, err := LoadAttr(fn, "__call__")
	if err != nil {
		t.Fatalf("read __call__: %v", err)
	}
	if call == nil || call == None {
		t.Fatal("__call__ should be a non-None callable")
	}
	got, err := CallKw(call, []Object{NewInt(9)}, nil, nil)
	if err != nil {
		t.Fatalf("call through __call__: %v", err)
	}
	if n, _ := AsInt(got); n != 9 {
		t.Errorf("echo.__call__(9) = %v, want 9", got)
	}
}
