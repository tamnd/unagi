package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestLocaleConvC checks _locale.localeconv() reports the standard C-locale
// values -- the ones CPython's real _locale returns for "C" on every platform.
func TestLocaleConvC(t *testing.T) {
	mo, err := ImportModule("_locale")
	if err != nil {
		t.Fatalf("import _locale: %v", err)
	}
	fn, err := objects.LoadAttr(mo, "localeconv")
	if err != nil {
		t.Fatalf("localeconv: %v", err)
	}
	d, err := objects.Call(fn, nil)
	if err != nil {
		t.Fatalf("localeconv(): %v", err)
	}
	str := func(key, want string) {
		t.Helper()
		v, err := objects.GetItem(d, objects.NewStr(key))
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if s, ok := objects.AsStr(v); !ok || s != want {
			t.Errorf("localeconv()[%q] = %v, want %q", key, v, want)
		}
	}
	num := func(key string, want int64) {
		t.Helper()
		v, err := objects.GetItem(d, objects.NewStr(key))
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if n, ok := objects.AsInt(v); !ok || n != want {
			t.Errorf("localeconv()[%q] = %v, want %d", key, v, want)
		}
	}
	str("decimal_point", ".")
	str("thousands_sep", "")
	str("currency_symbol", "")
	str("int_curr_symbol", "")
	str("positive_sign", "")
	str("negative_sign", "")
	num("frac_digits", 127)
	num("int_frac_digits", 127)
	num("p_sign_posn", 127)
	num("n_sign_posn", 127)
	// grouping and mon_grouping are empty lists in the C locale.
	for _, key := range []string{"grouping", "mon_grouping"} {
		v, err := objects.GetItem(d, objects.NewStr(key))
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if n, _ := objects.Len(v); n != 0 {
			t.Errorf("localeconv()[%q] len = %d, want 0", key, n)
		}
	}
}

// TestLocaleConstants checks the category constants, CHAR_MAX, and that Error is
// a subclassable Exception subclass.
func TestLocaleConstants(t *testing.T) {
	mo, err := ImportModule("_locale")
	if err != nil {
		t.Fatalf("import _locale: %v", err)
	}
	for _, c := range []struct {
		name string
		want int64
	}{
		{"CHAR_MAX", 127},
		{"LC_CTYPE", 0},
		{"LC_ALL", 6},
	} {
		v, err := objects.LoadAttr(mo, c.name)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if n, ok := objects.AsInt(v); !ok || n != c.want {
			t.Errorf("%s = %v, want %d", c.name, v, c.want)
		}
	}
	errCls, err := objects.LoadAttr(mo, "Error")
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	baseExc, _ := objects.ExcClassValue("Exception")
	if sub, _ := objects.IsSubclass(errCls, baseExc); sub != objects.True {
		t.Errorf("_locale.Error is not a subclass of Exception")
	}
}

// TestLocaleSetlocaleAndCollation checks setlocale for the portable locales and
// the C-locale strcoll/strxfrm behaviour.
func TestLocaleSetlocaleAndCollation(t *testing.T) {
	mo, err := ImportModule("_locale")
	if err != nil {
		t.Fatalf("import _locale: %v", err)
	}
	setlocale, err := objects.LoadAttr(mo, "setlocale")
	if err != nil {
		t.Fatalf("setlocale: %v", err)
	}
	lcAll, _ := objects.LoadAttr(mo, "LC_ALL")
	for _, loc := range []objects.Object{objects.None, objects.NewStr("C"), objects.NewStr("POSIX"), objects.NewStr("")} {
		r, err := objects.Call(setlocale, []objects.Object{lcAll, loc})
		if err != nil {
			t.Fatalf("setlocale(LC_ALL, %v): %v", loc, err)
		}
		if s, ok := objects.AsStr(r); !ok || s != "C" {
			t.Errorf("setlocale(LC_ALL, %v) = %v, want \"C\"", loc, r)
		}
	}
	// An unsupported locale raises _locale.Error.
	if _, err := objects.Call(setlocale, []objects.Object{lcAll, objects.NewStr("zz_ZZ.UTF-8")}); err == nil {
		t.Error("setlocale with an unsupported locale did not raise")
	}

	strcoll, _ := objects.LoadAttr(mo, "strcoll")
	for _, tc := range []struct {
		a, b string
		want int64
	}{
		{"ab", "ac", -1},
		{"abc", "abc", 0},
		{"b", "a", 1},
	} {
		r, err := objects.Call(strcoll, objs(objects.NewStr(tc.a), objects.NewStr(tc.b)))
		if err != nil {
			t.Fatalf("strcoll(%q,%q): %v", tc.a, tc.b, err)
		}
		if n, ok := objects.AsInt(r); !ok || n != tc.want {
			t.Errorf("strcoll(%q,%q) = %v, want %d", tc.a, tc.b, r, tc.want)
		}
	}
	strxfrm, _ := objects.LoadAttr(mo, "strxfrm")
	r, err := objects.Call(strxfrm, objs(objects.NewStr("hello")))
	if err != nil {
		t.Fatalf("strxfrm: %v", err)
	}
	if s, ok := objects.AsStr(r); !ok || s != "hello" {
		t.Errorf("strxfrm(\"hello\") = %v, want \"hello\"", r)
	}
}

// TestLocaleEmbeddedNull checks strcoll and strxfrm reject a string with an
// embedded NUL with ValueError, the way CPython's wide-char conversion does,
// since the underlying C functions take a NUL-terminated string.
func TestLocaleEmbeddedNull(t *testing.T) {
	mo, err := ImportModule("_locale")
	if err != nil {
		t.Fatalf("import _locale: %v", err)
	}
	strcoll, _ := objects.LoadAttr(mo, "strcoll")
	strxfrm, _ := objects.LoadAttr(mo, "strxfrm")

	isValueError := func(err error) bool {
		e, ok := err.(*objects.Exception)
		return ok && e.Kind == "ValueError"
	}

	// A NUL in either strcoll argument raises, so both conversion sites are
	// covered.
	if _, err := objects.Call(strcoll, objs(objects.NewStr("a\x00"), objects.NewStr("a"))); !isValueError(err) {
		t.Errorf("strcoll(NUL, a) error = %v, want ValueError", err)
	}
	if _, err := objects.Call(strcoll, objs(objects.NewStr("a"), objects.NewStr("a\x00"))); !isValueError(err) {
		t.Errorf("strcoll(a, NUL) error = %v, want ValueError", err)
	}
	if _, err := objects.Call(strxfrm, objs(objects.NewStr("a\x00"))); !isValueError(err) {
		t.Errorf("strxfrm(NUL) error = %v, want ValueError", err)
	}
}

// TestLocaleSetlocaleBadCategory checks a category integer outside the known LC_*
// range raises _locale.Error rather than reporting the C locale, matching the
// crasher guard CPython added in bug #7419.
func TestLocaleSetlocaleBadCategory(t *testing.T) {
	mo, err := ImportModule("_locale")
	if err != nil {
		t.Fatalf("import _locale: %v", err)
	}
	setlocale, _ := objects.LoadAttr(mo, "setlocale")
	errCls, _ := objects.LoadAttr(mo, "Error")

	isLocaleError := func(err error) bool {
		e, ok := err.(*objects.Exception)
		return ok && objects.ExcMatchesClass(e, errCls)
	}

	// Query mode (no locale) and set mode both reject an out-of-range category.
	if _, err := objects.Call(setlocale, objs(objects.NewInt(12345))); !isLocaleError(err) {
		t.Errorf("setlocale(12345) error = %v, want _locale.Error", err)
	}
	if _, err := objects.Call(setlocale, objs(objects.NewInt(-1))); !isLocaleError(err) {
		t.Errorf("setlocale(-1) error = %v, want _locale.Error", err)
	}
	if _, err := objects.Call(setlocale, objs(objects.NewInt(12345), objects.NewStr("C"))); !isLocaleError(err) {
		t.Errorf("setlocale(12345, C) error = %v, want _locale.Error", err)
	}
	// Every valid category still queries the C locale.
	for cat := int64(0); cat <= 6; cat++ {
		r, err := objects.Call(setlocale, objs(objects.NewInt(cat)))
		if err != nil {
			t.Fatalf("setlocale(%d): %v", cat, err)
		}
		if s, ok := objects.AsStr(r); !ok || s != "C" {
			t.Errorf("setlocale(%d) = %v, want \"C\"", cat, r)
		}
	}
}
