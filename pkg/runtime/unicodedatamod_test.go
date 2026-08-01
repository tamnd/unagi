package runtime

import (
	"strconv"
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestUnicodedataCategory checks category() against the CPython general
// category for one character from each of the 29 categories, the property
// computed for real from Go's unicode tables.
func TestUnicodedataCategory(t *testing.T) {
	cases := []struct {
		ch   string
		want string
	}{
		{"A", "Lu"}, {"z", "Ll"}, {"ǅ", "Lt"}, {"ʰ", "Lm"}, {"中", "Lo"},
		{"́", "Mn"}, {"ः", "Mc"}, {"⃝", "Me"},
		{"7", "Nd"}, {"Ⅻ", "Nl"}, {"½", "No"},
		{"_", "Pc"}, {"-", "Pd"}, {"(", "Ps"}, {")", "Pe"}, {"«", "Pi"}, {"»", "Pf"}, {"!", "Po"},
		{"+", "Sm"}, {"$", "Sc"}, {"^", "Sk"}, {"©", "So"},
		{" ", "Zs"}, {" ", "Zl"}, {" ", "Zp"},
		{"\x00", "Cc"}, {"\u200b", "Cf"}, {"\U000f0000", "Co"},
		{"\U0003fffd", "Cn"},
	}
	for _, c := range cases {
		got, err := udCategory([]objects.Object{objects.NewStr(c.ch)})
		if err != nil {
			t.Fatalf("category(%q): %v", c.ch, err)
		}
		if s, _ := objects.AsStr(got); s != c.want {
			t.Errorf("category(%q) = %q, want %q", c.ch, s, c.want)
		}
	}
}

// TestUnicodedataNumeric checks decimal/digit/numeric against values captured
// from the pinned python3.14 (the conformance oracle), now that all three read
// the pinned UCD tables rather than an ASCII-only path. It covers the three
// nested properties (a superscript has a digit and numeric value but no decimal;
// a fraction has a numeric value but no digit) and non-Latin scripts.
func TestUnicodedataNumeric(t *testing.T) {
	// decimal(chr) -> int, "" means no decimal value (ValueError without default).
	decimalCases := []struct{ ch, want string }{
		{"7", "7"}, {"٥", "5"}, {"५", "5"}, {"７", "7"},
		{"²", ""}, {"½", ""}, {"a", ""},
	}
	for _, c := range decimalCases {
		got, err := udDecimal([]objects.Object{objects.NewStr(c.ch)}, nil, nil)
		if c.want == "" {
			if ex, ok := err.(*objects.Exception); !ok || ex.Kind != objects.ValueError {
				t.Errorf("decimal(%q) = %v, want ValueError", c.ch, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("decimal(%q): %v", c.ch, err)
		}
		if objects.Repr(got) != c.want {
			t.Errorf("decimal(%q) = %s, want %s", c.ch, objects.Repr(got), c.want)
		}
	}

	// digit(chr) -> int; the superscripts and subscripts carry a digit value.
	digitCases := []struct{ ch, want string }{
		{"7", "7"}, {"²", "2"}, {"³", "3"}, {"₉", "9"},
		{"½", ""}, {"Ⅹ", ""},
	}
	for _, c := range digitCases {
		got, err := udDigit([]objects.Object{objects.NewStr(c.ch)}, nil, nil)
		if c.want == "" {
			if ex, ok := err.(*objects.Exception); !ok || ex.Kind != objects.ValueError {
				t.Errorf("digit(%q) = %v, want ValueError", c.ch, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("digit(%q): %v", c.ch, err)
		}
		if objects.Repr(got) != c.want {
			t.Errorf("digit(%q) = %s, want %s", c.ch, objects.Repr(got), c.want)
		}
	}

	// numeric(chr) -> float; the fractions, Roman numerals and CJK numerals carry
	// a numeric value with no digit value.
	numericCases := []struct{ ch, want string }{
		{"5", "5.0"}, {"½", "0.5"}, {"¼", "0.25"}, {"²", "2.0"},
		{"Ⅹ", "10.0"}, {"万", "10000.0"}, {"a", ""},
	}
	for _, c := range numericCases {
		got, err := udNumeric([]objects.Object{objects.NewStr(c.ch)}, nil, nil)
		if c.want == "" {
			if ex, ok := err.(*objects.Exception); !ok || ex.Kind != objects.ValueError {
				t.Errorf("numeric(%q) = %v, want ValueError", c.ch, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("numeric(%q): %v", c.ch, err)
		}
		if objects.Repr(got) != c.want {
			t.Errorf("numeric(%q) = %s, want %s", c.ch, objects.Repr(got), c.want)
		}
	}

	// With a default, a character that has no value returns the default.
	got, err := udDecimal([]objects.Object{objects.NewStr("a"), objects.NewInt(-1)}, nil, nil)
	if err != nil {
		t.Fatalf("decimal('a', -1): %v", err)
	}
	if objects.Repr(got) != "-1" {
		t.Errorf("decimal('a', -1) = %s, want -1", objects.Repr(got))
	}
}

// TestUnicodedataMirrored checks mirrored() against the pinned Bidi_Mirrored
// table: the brackets, angle quotes and math relations that flip under a
// right-to-left run report 1, and everything else reports 0.
func TestUnicodedataMirrored(t *testing.T) {
	cases := []struct {
		ch   string
		want int64
	}{
		{"(", 1}, {")", 1}, {"[", 1}, {"]", 1}, {"{", 1}, {"}", 1},
		{"<", 1}, {">", 1}, {"«", 1}, {"»", 1}, {"∫", 1},
		{"A", 0}, {"5", 0}, {"中", 0}, {" ", 0},
	}
	for _, c := range cases {
		got, err := udMirrored([]objects.Object{objects.NewStr(c.ch)})
		if err != nil {
			t.Fatalf("mirrored(%q): %v", c.ch, err)
		}
		if n, _ := objects.AsInt(got); n != c.want {
			t.Errorf("mirrored(%q) = %d, want %d", c.ch, n, c.want)
		}
	}
}

// TestUnicodedataVersion checks unidata_version reports the pinned UCD version
// and that category() answers from that same data: a code point assigned in
// Unicode 16.0 (the Garay script, new in this version) reports its real category
// rather than the Cn an older UCD would give.
func TestUnicodedataVersion(t *testing.T) {
	mo, err := ImportModule("unicodedata")
	if err != nil {
		t.Fatalf("import unicodedata: %v", err)
	}
	ver, err := objects.LoadAttr(mo, "unidata_version")
	if err != nil {
		t.Fatalf("unidata_version: %v", err)
	}
	if s, _ := objects.AsStr(ver); s != "16.0.0" {
		t.Errorf("unidata_version = %q, want 16.0.0", s)
	}
	// U+10D40 GARAY DIGIT ZERO is a decimal digit assigned in Unicode 16.0.
	got, err := udCategory([]objects.Object{objects.NewStr("\U00010D40")})
	if err != nil {
		t.Fatalf("category(U+10D40): %v", err)
	}
	if s, _ := objects.AsStr(got); s != "Nd" {
		t.Errorf("category(U+10D40) = %q, want Nd", s)
	}
}

// TestUnicodedataName checks name() against the pinned name database: an explicit
// name, an algorithmic CJK-ideograph name (min four hex digits, five for a
// supplementary code point) and an algorithmic Hangul syllable name, plus the
// ValueError a character with no name raises and the default that suppresses it.
func TestUnicodedataName(t *testing.T) {
	cases := []struct{ ch, want string }{
		{"A", "LATIN CAPITAL LETTER A"},
		{"中", "CJK UNIFIED IDEOGRAPH-4E2D"},
		{"㐀", "CJK UNIFIED IDEOGRAPH-3400"},
		{"\U00020000", "CJK UNIFIED IDEOGRAPH-20000"},
		{"가", "HANGUL SYLLABLE GA"},
		{"힣", "HANGUL SYLLABLE HIH"},
		{"\U00010D40", "GARAY DIGIT ZERO"},
	}
	for _, c := range cases {
		got, err := udName([]objects.Object{objects.NewStr(c.ch)}, nil, nil)
		if err != nil {
			t.Fatalf("name(%q): %v", c.ch, err)
		}
		if s, _ := objects.AsStr(got); s != c.want {
			t.Errorf("name(%q) = %q, want %q", c.ch, s, c.want)
		}
	}
	// a control character has no name: ValueError without a default, the default
	// with one.
	if _, err := udName([]objects.Object{objects.NewStr("\x00")}, nil, nil); err == nil {
		t.Errorf("name(U+0000) = no error, want ValueError")
	}
	got, err := udName([]objects.Object{objects.NewStr("\x00"), objects.NewStr("none")}, nil, nil)
	if err != nil {
		t.Fatalf("name(U+0000, 'none'): %v", err)
	}
	if s, _ := objects.AsStr(got); s != "none" {
		t.Errorf("name(U+0000, 'none') = %q, want none", s)
	}
}

// TestUnicodedataLookup checks lookup() against the pinned reverse of the name
// database: an explicit name, an algorithmic CJK and Hangul name, a name alias
// (NULL for U+0000, which has no name()), and a named sequence (which resolves to
// more than one character), plus the KeyError an undefined name raises and the
// rejection of a wrongly zero-padded algorithmic name.
func TestUnicodedataLookup(t *testing.T) {
	cases := []struct{ name, want string }{
		{"LATIN CAPITAL LETTER A", "A"},
		{"CJK UNIFIED IDEOGRAPH-4E2D", "中"},
		{"HANGUL SYLLABLE GA", "가"},
		{"NULL", "\x00"},
		{"LATIN CAPITAL LETTER A WITH MACRON AND GRAVE", "Ā̀"},
	}
	for _, c := range cases {
		got, err := udLookup([]objects.Object{objects.NewStr(c.name)})
		if err != nil {
			t.Fatalf("lookup(%q): %v", c.name, err)
		}
		if s, _ := objects.AsStr(got); s != c.want {
			t.Errorf("lookup(%q) = %q, want %q", c.name, s, c.want)
		}
	}
	for _, bad := range []string{"NO SUCH NAME", "CJK UNIFIED IDEOGRAPH-04E2D"} {
		_, err := udLookup([]objects.Object{objects.NewStr(bad)})
		if ex, ok := err.(*objects.Exception); !ok || ex.Kind != objects.KeyError {
			t.Errorf("lookup(%q) = %v, want KeyError", bad, err)
		}
	}
}

// TestUnicodedataBidirectional checks bidirectional() against the pinned
// Bidi_Class range table: Latin is L, Hebrew is R, Arabic is AL, a combining
// mark is NSM, a space is WS, the digits are EN, and an unassigned code point
// with no bidi class answers "".
func TestUnicodedataBidirectional(t *testing.T) {
	cases := []struct{ ch, want string }{
		{"A", "L"}, {"א", "R"}, {"ا", "AL"}, {"́", "NSM"},
		{" ", "WS"}, {"5", "EN"}, {"٠", "AN"}, {"(", "ON"},
		{"\U0003fffd", ""},
	}
	for _, c := range cases {
		got, err := udBidirectional([]objects.Object{objects.NewStr(c.ch)})
		if err != nil {
			t.Fatalf("bidirectional(%q): %v", c.ch, err)
		}
		if s, _ := objects.AsStr(got); s != c.want {
			t.Errorf("bidirectional(%q) = %q, want %q", c.ch, s, c.want)
		}
	}
}

// TestUnicodedataEastAsianWidth checks the wide-block heuristic: CJK is Wide,
// fullwidth forms are Fullwidth, ASCII is Narrow, and other text is Neutral.
func TestUnicodedataEastAsianWidth(t *testing.T) {
	cases := []struct{ ch, want string }{
		{"中", "W"}, {"Ａ", "F"}, {"A", "Na"}, {"é", "N"},
	}
	for _, c := range cases {
		got, err := udEastAsianWidth([]objects.Object{objects.NewStr(c.ch)})
		if err != nil {
			t.Fatalf("east_asian_width(%q): %v", c.ch, err)
		}
		if s, _ := objects.AsStr(got); s != c.want {
			t.Errorf("east_asian_width(%q) = %q, want %q", c.ch, s, c.want)
		}
	}
}

// hexRunes turns a comma-separated list of hex code points ("0065,0301") into
// the string it denotes, for the oracle-captured normalize golden cases.
func hexRunes(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	for h := range strings.SplitSeq(s, ",") {
		n, err := strconv.ParseInt(h, 16, 32)
		if err != nil {
			panic(err)
		}
		b.WriteRune(rune(n))
	}
	return b.String()
}

// TestUnicodedataNormalize checks normalize against a corpus captured from the
// pinned python3.14 (the conformance oracle), covering canonical precompose and
// decompose, compatibility folding, Hangul compose/decompose by arithmetic,
// canonical reordering of stacked marks, a singleton (U+212B), a blocked
// composition, and a non-BMP code point. Each pair (in, want) is that form's
// exact CPython output.
func TestUnicodedataNormalize(t *testing.T) {
	cases := []struct{ form, in, want string }{
		{"NFC", "00E9", "00E9"},
		{"NFD", "00E9", "0065,0301"},
		{"NFKC", "FB01", "0066,0069"},
		{"NFKD", "FB01", "0066,0069"},
		{"NFC", "D55C", "D55C"},
		{"NFD", "D55C", "1112,1161,11AB"},
		{"NFC", "AC01", "AC01"},
		{"NFD", "AC01", "1100,1161,11A8"},
		{"NFD", "1161", "1161"},
		{"NFC", "1112,1161,11AB", "D55C"},
		{"NFC", "0071,0323,0307", "0071,0323,0307"},
		{"NFD", "0071,0307,0323", "0071,0323,0307"},
		{"NFC", "1E0B,0323", "1E0D,0307"},
		{"NFD", "1E0B,0323", "0064,0323,0307"},
		{"NFC", "00C5", "00C5"},
		{"NFD", "00C5", "0041,030A"},
		{"NFKC", "FF12", "0032"},
		{"NFKD", "3300", "30A2,30CF,309A,30FC,30C8"},
		{"NFC", "006F,0066,0066,0069,0063,0065", "006F,0066,0066,0069,0063,0065"},
		{"NFC", "", ""},
		{"NFC", "0061,0300,0301,0062", "00E0,0301,0062"},
		{"NFC", "1D160", "1D158,1D165,1D16E"},
		{"NFKC", "1E9B", "1E61"},
	}
	for _, c := range cases {
		in, want := hexRunes(c.in), hexRunes(c.want)
		got, err := udNormalize([]objects.Object{objects.NewStr(c.form), objects.NewStr(in)})
		if err != nil {
			t.Fatalf("normalize(%s, %s): %v", c.form, c.in, err)
		}
		if s, _ := objects.AsStr(got); s != want {
			t.Errorf("normalize(%s, %s) = %q, want %q", c.form, c.in, s, want)
		}
		// is_normalized agrees with the fixed point of normalize.
		norm, err := udIsNormalized([]objects.Object{objects.NewStr(c.form), objects.NewStr(want)})
		if err != nil {
			t.Fatalf("is_normalized(%s, want): %v", c.form, err)
		}
		if b, _ := objects.AsBool(norm); !b {
			t.Errorf("is_normalized(%s, %q) = false, want true", c.form, want)
		}
	}

	// A short ASCII string is unchanged and reported normalized in every form.
	got, err := udNormalize([]objects.Object{objects.NewStr("NFC"), objects.NewStr("abc")})
	if err != nil {
		t.Fatalf("normalize('NFC', 'abc'): %v", err)
	}
	if s, _ := objects.AsStr(got); s != "abc" {
		t.Errorf("normalize('NFC', 'abc') = %q, want abc", s)
	}
	if _, err := udNormalize([]objects.Object{objects.NewStr("NFX"), objects.NewStr("abc")}); err == nil {
		t.Errorf("normalize with bad form should raise ValueError")
	}
}

// TestUnicodedataCombining checks combining() against the pinned class table:
// a starter is 0, and stacked marks report their true classes.
func TestUnicodedataCombining(t *testing.T) {
	cases := []struct {
		ch   string
		want int64
	}{
		{"a", 0}, {"́", 230}, {"̣", 220}, {"्", 9}, {"゙", 8},
	}
	for _, c := range cases {
		got, err := udCombining([]objects.Object{objects.NewStr(c.ch)})
		if err != nil {
			t.Fatalf("combining(%q): %v", c.ch, err)
		}
		if n, _ := objects.AsInt(got); n != c.want {
			t.Errorf("combining(%q) = %d, want %d", c.ch, n, c.want)
		}
	}
}

// TestUnicodedataDecomposition checks decomposition() returns the raw one-level
// string CPython does: a tagged compatibility mapping, an untagged canonical
// mapping, arithmetic Hangul jamo, and "" for a character with none.
func TestUnicodedataDecomposition(t *testing.T) {
	cases := []struct{ ch, want string }{
		{"À", "0041 0300"},
		{"ﬁ", "<compat> 0066 0069"},
		{" ", "<noBreak> 0020"},
		{"각", "1100 1161 11A8"},
		{"가", "1100 1161"},
		{"a", ""},
	}
	for _, c := range cases {
		got, err := udDecomposition([]objects.Object{objects.NewStr(c.ch)})
		if err != nil {
			t.Fatalf("decomposition(%q): %v", c.ch, err)
		}
		if s, _ := objects.AsStr(got); s != c.want {
			t.Errorf("decomposition(%q) = %q, want %q", c.ch, s, c.want)
		}
	}
}

// TestUnicodedataOneRuneError checks a multi-character argument is the
// TypeError the C module raises.
func TestUnicodedataOneRuneError(t *testing.T) {
	if _, err := udCategory([]objects.Object{objects.NewStr("ab")}); err == nil {
		t.Errorf("category('ab') should raise TypeError")
	} else if ex, ok := err.(*objects.Exception); !ok || ex.Kind != objects.TypeError {
		t.Errorf("category('ab') = %v, want TypeError", err)
	}
}

// TestUnicodedataUCD320 checks the ucd_3_2_0 accessor stringprep binds: it
// reports unidata_version '3.2.0' (matching CPython, the Unicode version RFC
// 3454 targets) and exposes the same category surface as the module, so a
// category-driven stringprep table answers correctly.
func TestUnicodedataUCD320(t *testing.T) {
	mo, err := ImportModule("unicodedata")
	if err != nil {
		t.Fatalf("import unicodedata: %v", err)
	}
	ucd, err := objects.LoadAttr(mo, "ucd_3_2_0")
	if err != nil {
		t.Fatalf("ucd_3_2_0: %v", err)
	}
	ver, err := objects.LoadAttr(ucd, "unidata_version")
	if err != nil {
		t.Fatalf("ucd_3_2_0.unidata_version: %v", err)
	}
	if s, _ := objects.AsStr(ver); s != "3.2.0" {
		t.Errorf("ucd_3_2_0.unidata_version = %q, want 3.2.0", s)
	}
	// A function stored on a namespace is called unbound, so category(chr) keeps
	// its one-argument shape.
	cat, err := objects.CallMethod(ucd, "category", []objects.Object{objects.NewStr("A")})
	if err != nil {
		t.Fatalf("ucd_3_2_0.category('A'): %v", err)
	}
	if s, _ := objects.AsStr(cat); s != "Lu" {
		t.Errorf("ucd_3_2_0.category('A') = %q, want Lu", s)
	}
}
