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

// TestStdErrorHandlers checks the standard non-strict codec error handlers
// resolve a UnicodeError into the (replacement, newpos) pair CPython's
// registered handlers produce: ignore drops the span, replace substitutes '?'
// or U+FFFD by direction, xmlcharrefreplace emits decimal character references,
// and backslashreplace escapes characters on encode and bytes on decode. The
// handlers read the bad span straight off the structured attributes.
func TestStdErrorHandlers(t *testing.T) {
	enc := NewUnicodeEncodeError("ascii", "aሴ\U0001F600b", 1, 3, "ordinal not in range(128)")
	dec := NewUnicodeDecodeError("ascii", []byte("a\xff\xfeb"), 1, 3, "ordinal not in range(128)")

	check := func(name string, got Object, err error, wantRepl string, wantPos int64) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: err %v", name, err)
		}
		tup, ok := got.(*tupleObject)
		if !ok || len(tup.elts) != 2 {
			t.Fatalf("%s: not a 2-tuple: %v", name, got)
		}
		if s, _ := AsStr(tup.elts[0]); s != wantRepl {
			t.Errorf("%s: replacement = %q, want %q", name, s, wantRepl)
		}
		if p, _ := AsInt(tup.elts[1]); p != wantPos {
			t.Errorf("%s: newpos = %d, want %d", name, p, wantPos)
		}
	}

	g, err := IgnoreErrors([]Object{enc})
	check("ignore encode", g, err, "", 3)

	g, err = ReplaceErrors([]Object{enc})
	check("replace encode", g, err, "??", 3)
	g, err = ReplaceErrors([]Object{dec})
	check("replace decode", g, err, "�", 3)

	g, err = XMLCharRefReplaceErrors([]Object{enc})
	check("xmlcharrefreplace encode", g, err, "&#4660;&#128512;", 3)

	g, err = BackslashReplaceErrors([]Object{enc})
	check("backslashreplace encode", g, err, `\u1234\U0001f600`, 3)
	g, err = BackslashReplaceErrors([]Object{dec})
	check("backslashreplace decode", g, err, `\xff\xfe`, 3)

	// xmlcharrefreplace is encode-only: a decode error is a TypeError.
	if _, err := XMLCharRefReplaceErrors([]Object{dec}); err == nil {
		t.Error("xmlcharrefreplace on decode error: want TypeError, got nil")
	}
	// A non-unicode-error argument is a TypeError for every handler.
	if _, err := IgnoreErrors([]Object{NewInt(1)}); err == nil {
		t.Error("ignore on non-unicode-error: want TypeError, got nil")
	}
}

// TestNameReplaceErrors checks the namereplace handler emits \N{NAME} for a
// character the name lookup resolves and falls back to the backslash escape for
// one it does not, over the whole bad span, and stays encode-only. The name
// lookup is the hook the unicodedata shim fills at init; the test installs a
// small stub so the objects package can exercise both paths on its own.
func TestNameReplaceErrors(t *testing.T) {
	prev := NameReplaceNameLookup
	defer func() { NameReplaceNameLookup = prev }()
	names := map[rune]string{
		0x1234:  "ETHIOPIC SYLLABLE SEE",
		0x1F600: "GRINNING FACE",
		0x00E9:  "LATIN SMALL LETTER E WITH ACUTE",
	}
	NameReplaceNameLookup = func(r rune) (string, bool) {
		n, ok := names[r]
		return n, ok
	}

	check := func(name string, got Object, err error, wantRepl string, wantPos int64) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: err %v", name, err)
		}
		tup, ok := got.(*tupleObject)
		if !ok || len(tup.elts) != 2 {
			t.Fatalf("%s: not a 2-tuple: %v", name, got)
		}
		if s, _ := AsStr(tup.elts[0]); s != wantRepl {
			t.Errorf("%s: replacement = %q, want %q", name, s, wantRepl)
		}
		if p, _ := AsInt(tup.elts[1]); p != wantPos {
			t.Errorf("%s: newpos = %d, want %d", name, p, wantPos)
		}
	}

	// Two named characters across the span.
	enc := NewUnicodeEncodeError("ascii", "aሴ\U0001F600b", 1, 3, "ordinal not in range(128)")
	g, err := NameReplaceErrors([]Object{enc})
	check("named span", g, err, `\N{ETHIOPIC SYLLABLE SEE}\N{GRINNING FACE}`, 3)

	// A code point with no name falls back to the backslashreplace escape.
	noName := NewUnicodeEncodeError("ascii", "ab", 1, 2, "ordinal not in range(128)")
	g, err = NameReplaceErrors([]Object{noName})
	check("no name fallback", g, err, `\x62`, 2)

	// A named BMP character below 0x100.
	acute := NewUnicodeEncodeError("ascii", "aéb", 1, 2, "ordinal not in range(128)")
	g, err = NameReplaceErrors([]Object{acute})
	check("named below 0x100", g, err, `\N{LATIN SMALL LETTER E WITH ACUTE}`, 2)

	// namereplace is encode-only: a decode error is a TypeError.
	dec := NewUnicodeDecodeError("ascii", []byte("a\xffb"), 1, 2, "ordinal not in range(128)")
	if _, err := NameReplaceErrors([]Object{dec}); err == nil {
		t.Error("namereplace on decode error: want TypeError, got nil")
	}

	// With the hook unset every character falls back to the escape.
	NameReplaceNameLookup = nil
	g, err = NameReplaceErrors([]Object{enc})
	check("no hook fallback", g, err, `\u1234\U0001f600`, 3)
}
