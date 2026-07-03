package objects

import "testing"

// Every expected string below was probed against python3.14 (3.14.6):
// str/repr across argument counts, the KeyError repr special case, and
// the final-line shapes.

func TestExceptionStrReprError(t *testing.T) {
	tests := []struct {
		name     string
		e        *Exception
		str      string
		repr     string
		errLine  string
		typeName string
	}{
		{"zero-args", NewException(ValueError, nil),
			"", "ValueError()", "ValueError", "ValueError"},
		{"one-str", NewException(ValueError, []Object{NewStr("boom")}),
			"boom", "ValueError('boom')", "ValueError: boom", "ValueError"},
		{"one-int", NewException(ValueError, []Object{NewInt(3)}),
			"3", "ValueError(3)", "ValueError: 3", "ValueError"},
		{"one-none", NewException(ValueError, []Object{None}),
			"None", "ValueError(None)", "ValueError: None", "ValueError"},
		{"two-args", NewException(TypeError, []Object{NewInt(1), NewInt(2)}),
			"(1, 2)", "TypeError(1, 2)", "TypeError: (1, 2)", "TypeError"},
		{"three-args", NewException("Exception", []Object{NewStr("a"), NewStr("b"), NewStr("c")}),
			"('a', 'b', 'c')", "Exception('a', 'b', 'c')", "Exception: ('a', 'b', 'c')", "Exception"},
		{"keyerror-str", NewException(KeyError, []Object{NewStr("k")}),
			"'k'", "KeyError('k')", "KeyError: 'k'", "KeyError"},
		{"keyerror-int", NewException(KeyError, []Object{NewInt(3)}),
			"3", "KeyError(3)", "KeyError: 3", "KeyError"},
		{"keyerror-zero", NewException(KeyError, nil),
			"", "KeyError()", "KeyError", "KeyError"},
		{"keyerror-two", NewException(KeyError, []Object{NewInt(1), NewInt(2)}),
			"(1, 2)", "KeyError(1, 2)", "KeyError: (1, 2)", "KeyError"},
		{"keyerror-tuple", NewException(KeyError, []Object{T(NewInt(1), NewInt(2))}),
			"(1, 2)", "KeyError((1, 2))", "KeyError: (1, 2)", "KeyError"},
		{"raise-ctor", Raise(ZeroDivisionError, "division by zero"),
			"division by zero", "ZeroDivisionError('division by zero')",
			"ZeroDivisionError: division by zero", "ZeroDivisionError"},
	}
	for _, tt := range tests {
		if got := Str(tt.e); got != tt.str {
			t.Errorf("%s: Str = %q, want %q", tt.name, got, tt.str)
		}
		if got := Repr(tt.e); got != tt.repr {
			t.Errorf("%s: Repr = %q, want %q", tt.name, got, tt.repr)
		}
		if got := tt.e.Error(); got != tt.errLine {
			t.Errorf("%s: Error = %q, want %q", tt.name, got, tt.errLine)
		}
		if got := tt.e.TypeName(); got != tt.typeName {
			t.Errorf("%s: TypeName = %q, want %q", tt.name, got, tt.typeName)
		}
		// bool(e) is True no matter the args, probed on 3.14.
		if !Truth(tt.e) {
			t.Errorf("%s: Truth = false, want true", tt.name)
		}
	}
}

func TestMatches(t *testing.T) {
	tests := []struct {
		kind, class string
		want        bool
	}{
		// Probed with issubclass on 3.14.
		{"ZeroDivisionError", "ZeroDivisionError", true},
		{"ZeroDivisionError", "ArithmeticError", true},
		{"ZeroDivisionError", "Exception", true},
		{"ZeroDivisionError", "BaseException", true},
		{"ZeroDivisionError", "LookupError", false},
		{"KeyError", "LookupError", true},
		{"IndexError", "LookupError", true},
		{"Exception", "ValueError", false},
		{"KeyboardInterrupt", "BaseException", true},
		{"KeyboardInterrupt", "Exception", false},
		// Warning classes.
		{"DeprecationWarning", "Warning", true},
		{"DeprecationWarning", "Exception", true},
		{"ResourceWarning", "Warning", true},
		// ExceptionGroup inherits from both BaseExceptionGroup and Exception.
		{"ExceptionGroup", "BaseExceptionGroup", true},
		{"ExceptionGroup", "Exception", true},
		{"ExceptionGroup", "BaseException", true},
		{"BaseExceptionGroup", "Exception", false},
		// OSError aliases in both positions.
		{"IOError", "OSError", true},
		{"OSError", "IOError", true},
		{"OSError", "EnvironmentError", true},
		{"ConnectionResetError", "OSError", true},
		{"ConnectionResetError", "IOError", true},
		// Unknown names never match.
		{"ValueError", "NotAClass", false},
		{"NotAClass", "Exception", false},
		{"NotAClass", "NotAClass", false},
	}
	for _, tt := range tests {
		if got := Matches(tt.kind, tt.class); got != tt.want {
			t.Errorf("Matches(%q, %q) = %v, want %v", tt.kind, tt.class, got, tt.want)
		}
	}
}

func TestIsExceptionClass(t *testing.T) {
	for _, name := range []string{
		"BaseException", "Exception", "ValueError", "KeyError",
		"ZeroDivisionError", "Warning", "DeprecationWarning",
		"ExceptionGroup", "BaseExceptionGroup", "OSError",
		"IOError", "EnvironmentError", "PythonFinalizationError",
	} {
		if !IsExceptionClass(name) {
			t.Errorf("IsExceptionClass(%q) = false", name)
		}
	}
	for _, name := range []string{"", "int", "str", "object", "Foo"} {
		if IsExceptionClass(name) {
			t.Errorf("IsExceptionClass(%q) = true", name)
		}
	}
}

func TestDictKeyErrorCarriesKey(t *testing.T) {
	d := mustDict(NewStr("a"), NewInt(1))
	_, err := GetItem(d, T(NewInt(1), NewInt(2)))
	e, ok := err.(*Exception)
	if !ok {
		t.Fatalf("expected *Exception, got %T", err)
	}
	if e.Kind != KeyError || len(e.Args) != 1 {
		t.Fatalf("KeyError shape = %q with %d args", e.Kind, len(e.Args))
	}
	// str(e) is the repr of the key, so the tuple key renders like CPython.
	if got := Str(e); got != "(1, 2)" {
		t.Errorf("Str = %q, want %q", got, "(1, 2)")
	}
	if got := e.Error(); got != "KeyError: (1, 2)" {
		t.Errorf("Error = %q, want %q", got, "KeyError: (1, 2)")
	}
}
