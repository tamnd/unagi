package runtime

import (
	"strings"

	"github.com/tamnd/unagi/pkg/objects"
)

// _locale is the C accelerator behind the public locale module. locale.py does
// `from _locale import *` and, only when that import fails, falls back to a
// minimal pure-Python emulation that supports the "C" locale alone. Providing
// _locale here makes `import _locale` succeed and lets locale.py bind the real
// module surface (localeconv, setlocale, strcoll, strxfrm, the category
// constants, CHAR_MAX and Error) rather than the emulation.
//
// unagi has no OS locale binding, so setlocale supports the portable "C"/"POSIX"
// locale exactly as the pure emulation does; the values reported are the
// standard C-locale values, which are identical on every platform. getencoding
// and nl_langinfo are deliberately not exposed: their results are
// platform-dependent, and locale.py already carries fallbacks for both, so
// leaving them out keeps behaviour deterministic without losing functionality.
//
// The category integers use the glibc ordering (LC_CTYPE=0 .. LC_ALL=6), the
// same values the pure emulation defined, so programs that pass these tokens
// around keep working unchanged. setlocale ignores the specific category since
// only the C locale is modelled.

var localeErrorClass objects.Object

func init() {
	moduleTable["_locale"] = &moduleEntry{builtin: true, exec: initLocale}
}

func initLocale(m *objects.Module) error {
	exc, ok := objects.ExcClassValue("Exception")
	if !ok {
		return objects.Raise(objects.RuntimeError, "_locale: Exception base is unavailable")
	}
	errCls, err := objects.NewClass("Error", "_locale.Error", []objects.Object{exc}, nil, nil, nil, nil)
	if err != nil {
		return err
	}
	localeErrorClass = errCls
	if err := objects.StoreAttr(m, "Error", errCls); err != nil {
		return err
	}

	// Category constants (glibc ordering) and CHAR_MAX.
	for _, c := range []struct {
		name string
		val  int64
	}{
		{"LC_CTYPE", 0},
		{"LC_NUMERIC", 1},
		{"LC_TIME", 2},
		{"LC_COLLATE", 3},
		{"LC_MONETARY", 4},
		{"LC_MESSAGES", 5},
		{"LC_ALL", 6},
		{"CHAR_MAX", 127},
	} {
		if err := objects.StoreAttr(m, c.name, objects.NewInt(c.val)); err != nil {
			return err
		}
	}

	for _, f := range []struct {
		name  string
		arity int
		fn    func([]objects.Object) (objects.Object, error)
	}{
		{"localeconv", 0, localeLocaleconv},
		{"strcoll", 2, localeStrcoll},
		{"strxfrm", 1, localeStrxfrm},
	} {
		if err := objects.StoreAttr(m, f.name, objects.NewFunc(f.name, f.arity, f.fn)); err != nil {
			return err
		}
	}
	// setlocale takes (category, locale=None), so it needs the kw-aware form.
	if err := objects.StoreAttr(m, "setlocale", objects.NewFuncKw("setlocale", localeSetlocale)); err != nil {
		return err
	}
	return nil
}

// localeLocaleconv is _locale.localeconv(): the numeric and monetary parameters
// of the current locale. Only the C locale is modelled, whose values are fixed
// by the C standard and identical on every platform. CHAR_MAX (127) in a numeric
// field means "unspecified"; empty strings and an empty grouping mean "no
// grouping / no symbol", matching CPython's C-locale localeconv exactly.
func localeLocaleconv(args []objects.Object) (objects.Object, error) {
	charMax := objects.NewInt(127)
	empty := objects.NewStr("")
	emptyList := func() objects.Object { return objects.NewList(nil) }
	keys := []objects.Object{
		objects.NewStr("decimal_point"),
		objects.NewStr("thousands_sep"),
		objects.NewStr("grouping"),
		objects.NewStr("int_curr_symbol"),
		objects.NewStr("currency_symbol"),
		objects.NewStr("mon_decimal_point"),
		objects.NewStr("mon_thousands_sep"),
		objects.NewStr("mon_grouping"),
		objects.NewStr("positive_sign"),
		objects.NewStr("negative_sign"),
		objects.NewStr("int_frac_digits"),
		objects.NewStr("frac_digits"),
		objects.NewStr("p_cs_precedes"),
		objects.NewStr("p_sep_by_space"),
		objects.NewStr("n_cs_precedes"),
		objects.NewStr("n_sep_by_space"),
		objects.NewStr("p_sign_posn"),
		objects.NewStr("n_sign_posn"),
	}
	vals := []objects.Object{
		objects.NewStr("."), // decimal_point
		empty,               // thousands_sep
		emptyList(),         // grouping
		empty,               // int_curr_symbol
		empty,               // currency_symbol
		empty,               // mon_decimal_point
		empty,               // mon_thousands_sep
		emptyList(),         // mon_grouping
		empty,               // positive_sign
		empty,               // negative_sign
		charMax,             // int_frac_digits
		charMax,             // frac_digits
		charMax,             // p_cs_precedes
		charMax,             // p_sep_by_space
		charMax,             // n_cs_precedes
		charMax,             // n_sep_by_space
		charMax,             // p_sign_posn
		charMax,             // n_sign_posn
	}
	return objects.NewDict(keys, vals)
}

// localeStrcoll is _locale.strcoll(a, b): a locale-aware string comparison. Under
// the C locale this is a plain code-point comparison, returning -1, 0 or 1. An
// embedded NUL raises ValueError the way CPython's wide-char conversion does,
// since the underlying C strcoll takes a NUL-terminated string.
func localeStrcoll(args []objects.Object) (objects.Object, error) {
	a, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "strcoll() argument 1 must be str, not %s", args[0].TypeName())
	}
	if strings.IndexByte(a, 0) >= 0 {
		return nil, objects.Raise(objects.ValueError, "embedded null character")
	}
	b, ok := objects.AsStr(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "strcoll() argument 2 must be str, not %s", args[1].TypeName())
	}
	if strings.IndexByte(b, 0) >= 0 {
		return nil, objects.Raise(objects.ValueError, "embedded null character")
	}
	return objects.NewInt(int64(strings.Compare(a, b))), nil
}

// localeStrxfrm is _locale.strxfrm(s): a transform whose plain comparison matches
// strcoll. The C locale needs no transform, so the string is returned unchanged.
// An embedded NUL raises ValueError, matching CPython's wide-char conversion.
func localeStrxfrm(args []objects.Object) (objects.Object, error) {
	s, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "strxfrm() argument 1 must be str, not %s", args[0].TypeName())
	}
	if strings.IndexByte(s, 0) >= 0 {
		return nil, objects.Raise(objects.ValueError, "embedded null character")
	}
	return objects.NewStr(s), nil
}

// localeSetlocale is _locale.setlocale(category, locale=None). Querying (locale
// None) and setting the portable "C"/"POSIX" locale both report "C"; any other
// locale is unsupported here and raises _locale.Error, exactly as the pure
// emulation does. The category is accepted but not otherwise used, since only the
// C locale is modelled.
func localeSetlocale(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	var category, loc objects.Object
	if len(pos) >= 1 {
		category = pos[0]
	}
	if len(pos) >= 2 {
		loc = pos[1]
	}
	for i, k := range kwNames {
		switch k {
		case "category":
			category = kwVals[i]
		case "locale":
			loc = kwVals[i]
		default:
			return nil, objects.Raise(objects.TypeError, "'%s' is an invalid keyword argument for setlocale()", k)
		}
	}
	if category == nil {
		return nil, objects.Raise(objects.TypeError, "setlocale() missing required argument 'category' (pos 1)")
	}
	cat, ok := objects.AsInt(category)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required (got type %s)", category.TypeName())
	}
	// The C setlocale returns NULL for a category outside the known LC_* range,
	// which CPython turns into Error. Only the constants this module defines
	// (LC_CTYPE=0 .. LC_ALL=6) are valid; anything else fails, and a query and a
	// set report the distinct messages CPython does.
	if cat < 0 || cat > 6 {
		if loc == nil || loc == objects.None {
			return nil, localeError("locale query failed")
		}
		return nil, localeError("unsupported locale setting")
	}
	if loc == nil || loc == objects.None {
		return objects.NewStr("C"), nil
	}
	name, ok := objects.AsStr(loc)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "locale must be str or None, not %s", loc.TypeName())
	}
	switch name {
	case "", "C", "POSIX":
		return objects.NewStr("C"), nil
	}
	return nil, localeError("unsupported locale setting")
}

// localeError builds a _locale.Error instance, the exception locale.py re-exports
// as locale.Error. It falls back to ValueError if the class is somehow unbound.
func localeError(msg string) error {
	if localeErrorClass != nil {
		if inst, err := objects.Call(localeErrorClass, []objects.Object{objects.NewStr(msg)}); err == nil {
			if e, ok := inst.(error); ok {
				return e
			}
		}
	}
	return objects.Raise(objects.ValueError, "%s", msg)
}
