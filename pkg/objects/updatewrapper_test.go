package objects

import "testing"

// uwFunc builds a plain function object under the given qualname whose impl
// returns its first argument, enough of a callable to carry attributes.
func uwFunc(qual string) Object {
	return NewFunction(qual, []Param{{Name: "x", Kind: ParamPlain}}, nil,
		func(a []Object) (Object, error) { return a[0], nil })
}

// names lifts Go strings into the tuple of name objects update_wrapper reads.
func names(ss ...string) Object {
	out := make([]Object, len(ss))
	for i, s := range ss {
		out[i] = NewStr(s)
	}
	return NewTuple(out)
}

func TestUpdateWrapperCopiesSlots(t *testing.T) {
	wrapped := WithFuncDoc(uwFunc("wrapped"), "the doc")
	wrapper := uwFunc("wrapper")

	assigned := names(WrapperAssignments...)
	updated := names(WrapperUpdates...)
	got, err := UpdateWrapper(wrapper, wrapped, assigned, updated)
	if err != nil {
		t.Fatalf("UpdateWrapper: %v", err)
	}
	if got != wrapper {
		t.Fatal("UpdateWrapper should return the wrapper")
	}
	for name, want := range map[string]string{
		"__name__":     "wrapped",
		"__qualname__": "wrapped",
		"__doc__":      "the doc",
	} {
		v, err := LoadAttr(wrapper, name)
		if err != nil {
			t.Fatalf("LoadAttr %s: %v", name, err)
		}
		if Str(v) != want {
			t.Errorf("%s = %q, want %q", name, Str(v), want)
		}
	}
	w, err := LoadAttr(wrapper, "__wrapped__")
	if err != nil {
		t.Fatalf("LoadAttr __wrapped__: %v", err)
	}
	if w != wrapped {
		t.Error("__wrapped__ should point at the wrapped function")
	}
}

func TestUpdateWrapperMergesDict(t *testing.T) {
	wrapped := uwFunc("wrapped")
	if err := StoreAttr(wrapped, "shared", NewStr("from_wrapped")); err != nil {
		t.Fatal(err)
	}
	if err := StoreAttr(wrapped, "only_w", NewInt(1)); err != nil {
		t.Fatal(err)
	}
	wrapper := uwFunc("wrapper")
	if err := StoreAttr(wrapper, "shared", NewStr("from_wrapper")); err != nil {
		t.Fatal(err)
	}
	if err := StoreAttr(wrapper, "only_p", NewInt(2)); err != nil {
		t.Fatal(err)
	}

	if _, err := UpdateWrapper(wrapper, wrapped, names(WrapperAssignments...), names(WrapperUpdates...)); err != nil {
		t.Fatalf("UpdateWrapper: %v", err)
	}
	// The wrapped value wins on a shared key, the wrapper keeps its own key, and
	// the wrapped-only key arrives.
	for name, want := range map[string]string{"shared": "from_wrapped", "only_p": "2", "only_w": "1"} {
		v, err := LoadAttr(wrapper, name)
		if err != nil {
			t.Fatalf("LoadAttr %s: %v", name, err)
		}
		if Str(v) != want {
			t.Errorf("%s = %q, want %q", name, Str(v), want)
		}
	}
}

func TestUpdateWrapperSkipsMissingAssigned(t *testing.T) {
	// A name the wrapped object does not carry is skipped rather than raising, so
	// the wrapper keeps its own value.
	wrapped := uwFunc("wrapped")
	wrapper := WithFuncDoc(uwFunc("wrapper"), "kept")
	if _, err := UpdateWrapper(wrapper, wrapped, names("__doc__", "missing_attr"), names()); err != nil {
		t.Fatalf("UpdateWrapper: %v", err)
	}
	v, err := LoadAttr(wrapper, "__doc__")
	if err != nil {
		t.Fatal(err)
	}
	// wrapped has no docstring, so its __doc__ is None and that copies across.
	if v != None {
		t.Errorf("__doc__ = %v, want None", v)
	}
}
