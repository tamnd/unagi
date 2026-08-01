package objects

import "testing"

// TestUnicodeErrorSplit checks the structured constructor split for the three
// unicode error types: the five-argument encode/decode form and the
// four-argument translate form fill the named slots, args stays whole, and str()
// renders CPython's single-position vs span wording with the escaped character or
// byte.
func TestUnicodeErrorSplit(t *testing.T) {
	for _, tc := range []struct {
		name     string
		kind     string
		args     []Object
		wantStr  string
		encoding Object
		object   Object
		start    Object
		end      Object
		reason   Object
	}{
		{
			name:     "decode single",
			kind:     "UnicodeDecodeError",
			args:     []Object{NewStr("utf-8"), NewBytes([]byte("abcdef")), NewInt(1), NewInt(2), NewStr("reason")},
			wantStr:  "'utf-8' codec can't decode byte 0x62 in position 1: reason",
			encoding: NewStr("utf-8"),
			object:   NewBytes([]byte("abcdef")),
			start:    NewInt(1),
			end:      NewInt(2),
			reason:   NewStr("reason"),
		},
		{
			name:     "decode span",
			kind:     "UnicodeDecodeError",
			args:     []Object{NewStr("utf-8"), NewBytes([]byte("abcdef")), NewInt(1), NewInt(4), NewStr("reason")},
			wantStr:  "'utf-8' codec can't decode bytes in position 1-3: reason",
			encoding: NewStr("utf-8"),
			object:   NewBytes([]byte("abcdef")),
			start:    NewInt(1),
			end:      NewInt(4),
			reason:   NewStr("reason"),
		},
		{
			name:     "encode single below 0x100",
			kind:     "UnicodeEncodeError",
			args:     []Object{NewStr("ascii"), NewStr("aéc"), NewInt(1), NewInt(2), NewStr("why")},
			wantStr:  "'ascii' codec can't encode character '\\xe9' in position 1: why",
			encoding: NewStr("ascii"),
			object:   NewStr("aéc"),
			start:    NewInt(1),
			end:      NewInt(2),
			reason:   NewStr("why"),
		},
		{
			name:     "encode single BMP",
			kind:     "UnicodeEncodeError",
			args:     []Object{NewStr("ascii"), NewStr("a中c"), NewInt(1), NewInt(2), NewStr("why")},
			wantStr:  "'ascii' codec can't encode character '\\u4e2d' in position 1: why",
			encoding: NewStr("ascii"),
			object:   NewStr("a中c"),
			start:    NewInt(1),
			end:      NewInt(2),
			reason:   NewStr("why"),
		},
		{
			name:     "encode single astral",
			kind:     "UnicodeEncodeError",
			args:     []Object{NewStr("ascii"), NewStr("a\U0001F600c"), NewInt(1), NewInt(2), NewStr("why")},
			wantStr:  "'ascii' codec can't encode character '\\U0001f600' in position 1: why",
			encoding: NewStr("ascii"),
			object:   NewStr("a\U0001F600c"),
			start:    NewInt(1),
			end:      NewInt(2),
			reason:   NewStr("why"),
		},
		{
			name:     "encode span",
			kind:     "UnicodeEncodeError",
			args:     []Object{NewStr("ascii"), NewStr("abcdef"), NewInt(1), NewInt(4), NewStr("why")},
			wantStr:  "'ascii' codec can't encode characters in position 1-3: why",
			encoding: NewStr("ascii"),
			object:   NewStr("abcdef"),
			start:    NewInt(1),
			end:      NewInt(4),
			reason:   NewStr("why"),
		},
		{
			name:     "translate single, no encoding",
			kind:     "UnicodeTranslateError",
			args:     []Object{NewStr("aéc"), NewInt(1), NewInt(2), NewStr("no")},
			wantStr:  "can't translate character '\\xe9' in position 1: no",
			encoding: None,
			object:   NewStr("aéc"),
			start:    NewInt(1),
			end:      NewInt(2),
			reason:   NewStr("no"),
		},
		{
			name:     "translate span",
			kind:     "UnicodeTranslateError",
			args:     []Object{NewStr("abcdef"), NewInt(1), NewInt(4), NewStr("no")},
			wantStr:  "can't translate characters in position 1-3: no",
			encoding: None,
			object:   NewStr("abcdef"),
			start:    NewInt(1),
			end:      NewInt(4),
			reason:   NewStr("no"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := NewException(tc.kind, tc.args)
			if !e.UEParsed {
				t.Fatal("UEParsed = false, want true")
			}
			if got := Str(e); got != tc.wantStr {
				t.Errorf("str = %q, want %q", got, tc.wantStr)
			}
			if got := attrOf(t, e, "encoding"); !sameObj(got, tc.encoding) {
				t.Errorf("encoding = %s, want %s", Repr(got), Repr(tc.encoding))
			}
			if got := attrOf(t, e, "object"); !sameObj(got, tc.object) {
				t.Errorf("object = %s, want %s", Repr(got), Repr(tc.object))
			}
			if got := attrOf(t, e, "start"); !sameObj(got, tc.start) {
				t.Errorf("start = %s, want %s", Repr(got), Repr(tc.start))
			}
			if got := attrOf(t, e, "end"); !sameObj(got, tc.end) {
				t.Errorf("end = %s, want %s", Repr(got), Repr(tc.end))
			}
			if got := attrOf(t, e, "reason"); !sameObj(got, tc.reason) {
				t.Errorf("reason = %s, want %s", Repr(got), Repr(tc.reason))
			}
			if got := len(e.Args); got != len(tc.args) {
				t.Errorf("len(args) = %d, want %d", got, len(tc.args))
			}
		})
	}
}

// TestUnicodeErrorNoSplit checks that argument shapes the constructor does not
// accept keep the generic exception form: the slots stay unset and str() is the
// plain message, never the codec-message form. A five-argument list on a plain
// ValueError also stays whole since the split is keyed on the unicode error kind.
func TestUnicodeErrorNoSplit(t *testing.T) {
	for _, tc := range []struct {
		name    string
		kind    string
		args    []Object
		wantStr string
	}{
		{"decode one arg message", "UnicodeDecodeError", []Object{NewStr("boom")}, "boom"},
		{"encode wrong count", "UnicodeEncodeError", []Object{NewStr("ascii"), NewStr("x"), NewInt(0)}, "('ascii', 'x', 0)"},
		{"translate wrong count", "UnicodeTranslateError", []Object{NewStr("x"), NewInt(0)}, "('x', 0)"},
		{"five args on plain ValueError", "ValueError", []Object{NewStr("a"), NewStr("b"), NewInt(0), NewInt(1), NewStr("r")}, "('a', 'b', 0, 1, 'r')"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := NewException(tc.kind, tc.args)
			if e.UEParsed {
				t.Fatal("UEParsed = true, want false")
			}
			if got := Str(e); got != tc.wantStr {
				t.Errorf("str = %q, want %q", got, tc.wantStr)
			}
		})
	}
}

// TestNewUnicodeErrorConstructors checks the runtime codec raise helpers build
// the structured five-tuple form: the whole input is the object, the span
// indexes into it, the named attributes read back, and str() renders CPython's
// codec-message wording. These are the path the multibyte codecs raise on, so
// a caught error carries start/end/object the way an error handler needs.
func TestNewUnicodeErrorConstructors(t *testing.T) {
	d := NewUnicodeDecodeError("euc_jp", []byte("abc\xff"), 3, 4, "incomplete multibyte sequence")
	if !d.UEParsed {
		t.Fatal("decode UEParsed = false")
	}
	if got := Str(d); got != "'euc_jp' codec can't decode byte 0xff in position 3: incomplete multibyte sequence" {
		t.Errorf("decode str = %q", got)
	}
	if s, _ := AsInt(d.UEStart); s != 3 {
		t.Errorf("decode start = %d, want 3", s)
	}
	if got, ok := unicodeErrorAttr(d, "object"); !ok || Str(got) != "b'abc\\xff'" {
		t.Errorf("decode object = %v ok=%v", got, ok)
	}

	e := NewUnicodeEncodeError("gb2312", "abc\U0001F600def", 3, 4, "illegal multibyte sequence")
	if !e.UEParsed {
		t.Fatal("encode UEParsed = false")
	}
	if got := Str(e); got != "'gb2312' codec can't encode character '\\U0001f600' in position 3: illegal multibyte sequence" {
		t.Errorf("encode str = %q", got)
	}
	if enc, ok := unicodeErrorAttr(e, "encoding"); !ok || Str(enc) != "gb2312" {
		t.Errorf("encode encoding = %v ok=%v", enc, ok)
	}
	// The object is the whole input string, so indexing at start names the bad
	// code point.
	if r, ok := runeAt(e.UEObject, 3); !ok || r != 0x1F600 {
		t.Errorf("encode object[3] = %U ok=%v", r, ok)
	}
}
