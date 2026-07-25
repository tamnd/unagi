package runtime

import (
	"unicode"

	"github.com/tamnd/unagi/pkg/objects"
)

// unicodedata is a built-in module. CPython implements it as a C extension over
// the bundled Unicode Character Database; the runtime provides the same surface
// in Go under the import name so `import unicodedata` (and the modules that
// import it, pdb through _pyrepl.utils and doctest through pdb) load.
//
// The general category is computed for real from Go's `unicode` package, which
// tracks the same property, so category() agrees with CPython character for
// character. The rest of the UCD (canonical combining class, bidi class, the
// name database, and the decomposition tables that back normalize) is not part
// of the Go standard library and is not bundled here, so those functions carry
// best-effort implementations where a stdlib range answers and spec-shaped
// fallbacks otherwise, each documented on the function. unidata_version reports
// the UCD version Go actually ships rather than claiming CPython's, so a caller
// that inspects it is not misled.

func init() {
	moduleTable["unicodedata"] = &moduleEntry{builtin: true, exec: initUnicodedata}
}

func initUnicodedata(m *objects.Module) error {
	for _, e := range []struct {
		name string
		obj  objects.Object
	}{
		{"category", objects.NewFunc("category", 1, udCategory)},
		{"combining", objects.NewFunc("combining", 1, udCombining)},
		{"east_asian_width", objects.NewFunc("east_asian_width", 1, udEastAsianWidth)},
		{"bidirectional", objects.NewFunc("bidirectional", 1, udBidirectional)},
		{"mirrored", objects.NewFunc("mirrored", 1, udMirrored)},
		{"decomposition", objects.NewFunc("decomposition", 1, udDecomposition)},
		{"decimal", objects.NewFuncKw("decimal", udDecimal)},
		{"digit", objects.NewFuncKw("digit", udDigit)},
		{"numeric", objects.NewFuncKw("numeric", udNumeric)},
		{"name", objects.NewFuncKw("name", udName)},
		{"lookup", objects.NewFunc("lookup", 1, udLookup)},
		{"normalize", objects.NewFunc("normalize", 2, udNormalize)},
		{"is_normalized", objects.NewFunc("is_normalized", 2, udIsNormalized)},
	} {
		if err := objects.StoreAttr(m, e.name, e.obj); err != nil {
			return err
		}
	}
	if err := objects.StoreAttr(m, "unidata_version", objects.NewStr(unicode.Version)); err != nil {
		return err
	}
	return nil
}

// oneRune pulls the single code point an argument must be, matching the C
// module's "need a single Unicode character" TypeError.
func oneRune(fn string, o objects.Object) (rune, error) {
	s, ok := objects.AsStr(o)
	if !ok {
		return 0, objects.Raise(objects.TypeError,
			"%s() argument 1 must be a unicode character, not %s", fn, o.TypeName())
	}
	runes := []rune(s)
	if len(runes) != 1 {
		return 0, objects.Raise(objects.TypeError,
			"%s() argument 1 must be a unicode character, not str", fn)
	}
	return runes[0], nil
}

// pyCategories lists the 29 Unicode general categories in a fixed order. Go's
// unicode.Categories map also carries an "LC" cased-letter supergroup that is
// not a Python general category, so an explicit list is used rather than
// ranging the map, which would let a cased letter match "LC" first.
var pyCategories = []string{
	"Lu", "Ll", "Lt", "Lm", "Lo",
	"Mn", "Mc", "Me",
	"Nd", "Nl", "No",
	"Pc", "Pd", "Ps", "Pe", "Pi", "Pf", "Po",
	"Sm", "Sc", "Sk", "So",
	"Zs", "Zl", "Zp",
	"Cc", "Cf", "Cs", "Co",
}

// runeCategory returns the CPython general category code, "Cn" for an
// unassigned code point, computed from Go's unicode tables.
func runeCategory(r rune) string {
	for _, code := range pyCategories {
		if tab := unicode.Categories[code]; tab != nil && unicode.Is(tab, r) {
			return code
		}
	}
	return "Cn"
}

// udCategory is unicodedata.category(chr), computed for real.
func udCategory(args []objects.Object) (objects.Object, error) {
	r, err := oneRune("category", args[0])
	if err != nil {
		return nil, err
	}
	return objects.NewStr(runeCategory(r)), nil
}

// udCombining is unicodedata.combining(chr), the canonical combining class.
// The class numbers live in the UCD, which the Go standard library does not
// expose, so a non-mark answers 0 (its true value) and a combining mark also
// answers 0 rather than a guessed class; callers that only test truthiness for
// "is this a combining mark" should prefer category() starting with "M".
func udCombining(args []objects.Object) (objects.Object, error) {
	if _, err := oneRune("combining", args[0]); err != nil {
		return nil, err
	}
	return objects.NewInt(0), nil
}

// wideRanges are the East Asian Wide and Fullwidth blocks. Go carries no
// East_Asian_Width property, so east_asian_width answers from the common wide
// blocks (CJK, Hangul, kana, fullwidth forms, emoji) and returns "N" elsewhere.
// This is a heuristic over the dominant cases, not the full UCD width property.
var wideRanges = []struct{ lo, hi rune }{
	{0x1100, 0x115F},   // Hangul Jamo
	{0x2E80, 0x303E},   // CJK radicals, Kangxi, CJK symbols
	{0x3041, 0x33FF},   // kana, CJK compatibility
	{0x3400, 0x4DBF},   // CJK ext A
	{0x4E00, 0x9FFF},   // CJK unified
	{0xA000, 0xA4CF},   // Yi
	{0xAC00, 0xD7A3},   // Hangul syllables
	{0xF900, 0xFAFF},   // CJK compatibility ideographs
	{0xFE30, 0xFE4F},   // CJK compatibility forms
	{0x1F300, 0x1FAFF}, // emoji and symbols
	{0x20000, 0x3FFFD}, // CJK ext B and beyond
}

// fullwidthRanges are the Fullwidth ("F") forms.
var fullwidthRanges = []struct{ lo, hi rune }{
	{0xFF00, 0xFF60},
	{0xFFE0, 0xFFE6},
}

func inRanges(r rune, rs []struct{ lo, hi rune }) bool {
	for _, x := range rs {
		if r >= x.lo && r <= x.hi {
			return true
		}
	}
	return false
}

// udEastAsianWidth is unicodedata.east_asian_width(chr), a wide-block heuristic.
func udEastAsianWidth(args []objects.Object) (objects.Object, error) {
	r, err := oneRune("east_asian_width", args[0])
	if err != nil {
		return nil, err
	}
	switch {
	case inRanges(r, fullwidthRanges):
		return objects.NewStr("F"), nil
	case inRanges(r, wideRanges):
		return objects.NewStr("W"), nil
	case r >= 0x20 && r <= 0x7E:
		// Printable ASCII is Narrow, the width traceback and _pyrepl assume for
		// Latin text; a control point and everything outside the wide blocks
		// falls to Neutral.
		return objects.NewStr("Na"), nil
	default:
		return objects.NewStr("N"), nil
	}
}

// udBidirectional is unicodedata.bidirectional(chr). The bidi class table is
// UCD data Go does not carry, so this returns "" for an unassigned point and
// "L" otherwise, the strong-left-to-right default; it is a stub for callers
// that need the full bidi property (stringprep's checks depend on the real
// table and are not served by this).
func udBidirectional(args []objects.Object) (objects.Object, error) {
	r, err := oneRune("bidirectional", args[0])
	if err != nil {
		return nil, err
	}
	if runeCategory(r) == "Cn" {
		return objects.NewStr(""), nil
	}
	return objects.NewStr("L"), nil
}

// udMirrored is unicodedata.mirrored(chr). The Bidi_Mirrored property is UCD
// data Go does not carry, so this reports 0 for every character.
func udMirrored(args []objects.Object) (objects.Object, error) {
	if _, err := oneRune("mirrored", args[0]); err != nil {
		return nil, err
	}
	return objects.NewInt(0), nil
}

// udDecomposition is unicodedata.decomposition(chr). The decomposition mapping
// is UCD data Go does not carry, so this reports "" (no decomposition) for
// every character, correct for the majority that have none.
func udDecomposition(args []objects.Object) (objects.Object, error) {
	if _, err := oneRune("decomposition", args[0]); err != nil {
		return nil, err
	}
	return objects.NewStr(""), nil
}

// numericArg pulls (chr, hasDefault, default) from a decimal/digit/numeric/name
// style call, which is func(chr[, default]).
func numericArg(fn string, pos []objects.Object, kwNames []string) (rune, bool, objects.Object, error) {
	if len(kwNames) > 0 {
		return 0, false, nil, objects.Raise(objects.TypeError, "%s() takes no keyword arguments", fn)
	}
	if len(pos) < 1 || len(pos) > 2 {
		return 0, false, nil, objects.Raise(objects.TypeError,
			"%s() takes one or two arguments", fn)
	}
	r, err := oneRune(fn, pos[0])
	if err != nil {
		return 0, false, nil, err
	}
	if len(pos) == 2 {
		return r, true, pos[1], nil
	}
	return r, false, nil, nil
}

// asciiDigit returns the decimal value 0..9 of an ASCII digit, or -1.
func asciiDigit(r rune) int {
	if r >= '0' && r <= '9' {
		return int(r - '0')
	}
	return -1
}

// udDecimal is unicodedata.decimal(chr[, default]). The decimal-value table is
// UCD data Go does not carry in full, so ASCII digits answer their real value
// and any other character falls to the default, or the ValueError CPython
// raises when a character has no decimal value and no default is given.
func udDecimal(pos []objects.Object, kwNames []string, _ []objects.Object) (objects.Object, error) {
	r, hasDef, def, err := numericArg("decimal", pos, kwNames)
	if err != nil {
		return nil, err
	}
	if v := asciiDigit(r); v >= 0 {
		return objects.NewInt(int64(v)), nil
	}
	if hasDef {
		return def, nil
	}
	return nil, objects.Raise(objects.ValueError, "not a decimal")
}

// udDigit is unicodedata.digit(chr[, default]), the same policy as decimal.
func udDigit(pos []objects.Object, kwNames []string, _ []objects.Object) (objects.Object, error) {
	r, hasDef, def, err := numericArg("digit", pos, kwNames)
	if err != nil {
		return nil, err
	}
	if v := asciiDigit(r); v >= 0 {
		return objects.NewInt(int64(v)), nil
	}
	if hasDef {
		return def, nil
	}
	return nil, objects.Raise(objects.ValueError, "not a digit")
}

// udNumeric is unicodedata.numeric(chr[, default]). It returns a float; ASCII
// digits answer their value and anything else falls to the default or the
// ValueError CPython raises.
func udNumeric(pos []objects.Object, kwNames []string, _ []objects.Object) (objects.Object, error) {
	r, hasDef, def, err := numericArg("numeric", pos, kwNames)
	if err != nil {
		return nil, err
	}
	if v := asciiDigit(r); v >= 0 {
		return objects.NewFloat(float64(v)), nil
	}
	if hasDef {
		return def, nil
	}
	return nil, objects.Raise(objects.ValueError, "not a numeric character")
}

// udName is unicodedata.name(chr[, default]). The name database is UCD data Go
// does not carry, so this returns the default when one is given and otherwise
// raises the ValueError CPython raises for a character with no name.
func udName(pos []objects.Object, kwNames []string, _ []objects.Object) (objects.Object, error) {
	_, hasDef, def, err := numericArg("name", pos, kwNames)
	if err != nil {
		return nil, err
	}
	if hasDef {
		return def, nil
	}
	return nil, objects.Raise(objects.ValueError, "no such name")
}

// udLookup is unicodedata.lookup(name). Without the name database it cannot
// resolve a character, so it raises the KeyError CPython raises for an
// unknown name.
func udLookup(args []objects.Object) (objects.Object, error) {
	name, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "lookup() argument must be str")
	}
	return nil, objects.Raise(objects.KeyError, "undefined character name '%s'", name)
}

// normForms are the four normalization forms normalize accepts.
var normForms = map[string]bool{"NFC": true, "NFD": true, "NFKC": true, "NFKD": true}

// isASCII reports whether every code point is below U+0080, the range every
// normalization form leaves unchanged.
func isASCII(s string) bool {
	for _, r := range s {
		if r >= 0x80 {
			return false
		}
	}
	return true
}

// udNormalize is unicodedata.normalize(form, unistr). The decomposition and
// composition tables are UCD data Go does not carry, so a pure-ASCII string,
// which every form leaves unchanged, is returned as-is and a string with
// non-ASCII content raises rather than returning a silently wrong result.
func udNormalize(args []objects.Object) (objects.Object, error) {
	form, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "normalize() argument 1 must be str")
	}
	s, ok := objects.AsStr(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "normalize() argument 2 must be str")
	}
	if !normForms[form] {
		return nil, objects.Raise(objects.ValueError, "invalid normalization form")
	}
	if isASCII(s) {
		return objects.NewStr(s), nil
	}
	return nil, objects.Raise("NotImplementedError",
		"unicodedata.normalize on non-ASCII text needs the Unicode decomposition database, which this runtime does not bundle")
}

// udIsNormalized is unicodedata.is_normalized(form, unistr), the same ASCII
// policy as normalize: ASCII is normalized in every form, non-ASCII cannot be
// answered without the decomposition database.
func udIsNormalized(args []objects.Object) (objects.Object, error) {
	form, ok := objects.AsStr(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "is_normalized() argument 1 must be str")
	}
	s, ok := objects.AsStr(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "is_normalized() argument 2 must be str")
	}
	if !normForms[form] {
		return nil, objects.Raise(objects.ValueError, "invalid normalization form")
	}
	if isASCII(s) {
		return objects.NewBool(true), nil
	}
	return nil, objects.Raise("NotImplementedError",
		"unicodedata.is_normalized on non-ASCII text needs the Unicode decomposition database, which this runtime does not bundle")
}
