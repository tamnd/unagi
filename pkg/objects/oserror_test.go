package objects

import "testing"

// sameObj compares two objects by repr, enough for the int/str/None values these
// tests assert on without reaching for the full rich-comparison machinery.
func sameObj(a, b Object) bool { return Repr(a) == Repr(b) }

// attrOf reads an exception attribute the way LoadAttr does, failing the test on
// an unexpected AttributeError so the split-slot reads can be asserted directly.
func attrOf(t *testing.T, e *Exception, name string) Object {
	t.Helper()
	v, err := excLoadAttr(e, name)
	if err != nil {
		t.Fatalf("%s.%s: %v", e.Kind, name, err)
	}
	return v
}

// TestOSErrorSplit checks the 2..5-argument constructor split: errno and
// strerror always fill, filename and filename2 fill when present, the winerror
// slot (arg 3) is dropped, args collapses to the (errno, strerror) pair, and
// str() renders the "[Errno N] strerror: 'filename' -> 'filename2'" form.
func TestOSErrorSplit(t *testing.T) {
	for _, tc := range []struct {
		name     string
		args     []Object
		wantStr  string
		errno    Object
		strerror Object
		filename Object
		fname2   Object
		wantArgN int
	}{
		{
			name:     "errno+strerror",
			args:     []Object{NewInt(2), NewStr("No such file or directory")},
			wantStr:  "[Errno 2] No such file or directory",
			errno:    NewInt(2),
			strerror: NewStr("No such file or directory"),
			filename: None,
			fname2:   None,
			wantArgN: 2,
		},
		{
			name:     "with filename",
			args:     []Object{NewInt(2), NewStr("nope"), NewStr("x.txt")},
			wantStr:  "[Errno 2] nope: 'x.txt'",
			errno:    NewInt(2),
			strerror: NewStr("nope"),
			filename: NewStr("x.txt"),
			fname2:   None,
			wantArgN: 2,
		},
		{
			name:     "winerror dropped, filename2 kept",
			args:     []Object{NewInt(2), NewStr("msg"), NewStr("src"), NewInt(0), NewStr("dst")},
			wantStr:  "[Errno 2] msg: 'src' -> 'dst'",
			errno:    NewInt(2),
			strerror: NewStr("msg"),
			filename: NewStr("src"),
			fname2:   NewStr("dst"),
			wantArgN: 2,
		},
		{
			name:     "non-int errno still splits, no remap",
			args:     []Object{NewStr("notanint"), NewStr("msg")},
			wantStr:  "[Errno notanint] msg",
			errno:    NewStr("notanint"),
			strerror: NewStr("msg"),
			filename: None,
			fname2:   None,
			wantArgN: 2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := NewException("OSError", tc.args)
			if !e.OSParsed {
				t.Fatal("OSParsed = false, want true")
			}
			if got := Str(e); got != tc.wantStr {
				t.Errorf("str = %q, want %q", got, tc.wantStr)
			}
			if got := attrOf(t, e, "errno"); !sameObj(got, tc.errno) {
				t.Errorf("errno = %s, want %s", Repr(got), Repr(tc.errno))
			}
			if got := attrOf(t, e, "strerror"); !sameObj(got, tc.strerror) {
				t.Errorf("strerror = %s, want %s", Repr(got), Repr(tc.strerror))
			}
			if got := attrOf(t, e, "filename"); !sameObj(got, tc.filename) {
				t.Errorf("filename = %s, want %s", Repr(got), Repr(tc.filename))
			}
			if got := attrOf(t, e, "filename2"); !sameObj(got, tc.fname2) {
				t.Errorf("filename2 = %s, want %s", Repr(got), Repr(tc.fname2))
			}
			if got := len(e.Args); got != tc.wantArgN {
				t.Errorf("len(args) = %d, want %d", got, tc.wantArgN)
			}
		})
	}
}

// TestOSErrorNoSplit checks the argument counts CPython leaves whole: zero, one,
// and more than five arguments keep the generic BaseException shape — args stay
// intact, the slots read None, and str is the empty/one-arg/tuple form, never the
// "[Errno ...]" form.
func TestOSErrorNoSplit(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []Object
		wantStr string
	}{
		{"zero args", nil, ""},
		{"one arg", []Object{NewStr("boom")}, "boom"},
		{"six args", []Object{NewInt(1), NewInt(2), NewInt(3), NewInt(4), NewInt(5), NewInt(6)}, "(1, 2, 3, 4, 5, 6)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := NewException("OSError", tc.args)
			if e.OSParsed {
				t.Error("OSParsed = true, want false")
			}
			if got := Str(e); got != tc.wantStr {
				t.Errorf("str = %q, want %q", got, tc.wantStr)
			}
			if got := attrOf(t, e, "errno"); got != None {
				t.Errorf("errno = %s, want None", Repr(got))
			}
			if got := attrOf(t, e, "filename"); got != None {
				t.Errorf("filename = %s, want None", Repr(got))
			}
			if got := len(e.Args); got != len(tc.args) {
				t.Errorf("len(args) = %d, want %d", got, len(tc.args))
			}
		})
	}
}

// TestOSErrorRemap checks the errno -> subclass remap that the platform hook
// drives: a bare OSError with a mapped integer errno becomes that subclass, an
// unmapped or non-int errno stays OSError, and a directly built subclass keeps
// its own class while still splitting its arguments.
func TestOSErrorRemap(t *testing.T) {
	prev := OSErrorSubclass
	OSErrorSubclass = func(errno int64) (string, bool) {
		if errno == 2 {
			return "FileNotFoundError", true
		}
		return "", false
	}
	defer func() { OSErrorSubclass = prev }()

	e := NewException("OSError", []Object{NewInt(2), NewStr("nope"), NewStr("x")})
	if e.Kind != "FileNotFoundError" {
		t.Errorf("bare OSError(2, ...) Kind = %q, want FileNotFoundError", e.Kind)
	}
	if got := attrOf(t, e, "filename"); !sameObj(got, NewStr("x")) {
		t.Errorf("remapped filename = %s, want 'x'", Repr(got))
	}

	if e := NewException("OSError", []Object{NewInt(99), NewStr("weird")}); e.Kind != "OSError" {
		t.Errorf("unmapped errno Kind = %q, want OSError", e.Kind)
	}
	if e := NewException("OSError", []Object{NewStr("nope"), NewStr("msg")}); e.Kind != "OSError" {
		t.Errorf("non-int errno Kind = %q, want OSError", e.Kind)
	}

	// A directly constructed subclass never remaps but still splits its args.
	direct := NewException("FileNotFoundError", []Object{NewInt(2), NewStr("nope"), NewStr("z")})
	if direct.Kind != "FileNotFoundError" {
		t.Errorf("direct subclass Kind = %q", direct.Kind)
	}
	if got := attrOf(t, direct, "errno"); !sameObj(got, NewInt(2)) {
		t.Errorf("direct subclass errno = %s, want 2", Repr(got))
	}
}

// TestOSErrorAttrsOnlyOnFamily checks the four members are an OSError-family
// concept: a non-OSError exception has no errno attribute, and a user value
// written onto an OSError's dict wins over the split slot.
func TestOSErrorAttrsOnlyOnFamily(t *testing.T) {
	ve := NewException(ValueError, []Object{NewInt(2), NewStr("msg")})
	if _, err := excLoadAttr(ve, "errno"); err == nil {
		t.Error("ValueError.errno did not raise AttributeError")
	}

	e := NewException("OSError", []Object{NewInt(2), NewStr("msg")})
	if _, err := excStoreAttr(e, "errno", NewInt(42)); err != nil {
		t.Fatalf("store errno: %v", err)
	}
	if got := attrOf(t, e, "errno"); !sameObj(got, NewInt(42)) {
		t.Errorf("overridden errno = %s, want 42", Repr(got))
	}
}
