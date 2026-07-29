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

// TestUnicodedataNumeric checks the ASCII digit paths and the ValueError a
// character with no value and no default raises.
func TestUnicodedataNumeric(t *testing.T) {
	dec, err := udDecimal([]objects.Object{objects.NewStr("7")}, nil, nil)
	if err != nil {
		t.Fatalf("decimal('7'): %v", err)
	}
	if objects.Repr(dec) != "7" {
		t.Errorf("decimal('7') = %s, want 7", objects.Repr(dec))
	}
	num, err := udNumeric([]objects.Object{objects.NewStr("5")}, nil, nil)
	if err != nil {
		t.Fatalf("numeric('5'): %v", err)
	}
	if objects.Repr(num) != "5.0" {
		t.Errorf("numeric('5') = %s, want 5.0", objects.Repr(num))
	}
	// A letter has no decimal value, so with no default it raises ValueError.
	if _, err := udDecimal([]objects.Object{objects.NewStr("a")}, nil, nil); err == nil {
		t.Errorf("decimal('a') without default should raise")
	} else if ex, ok := err.(*objects.Exception); !ok || ex.Kind != objects.ValueError {
		t.Errorf("decimal('a') = %v, want ValueError", err)
	}
	// With a default, the default comes back.
	got, err := udDecimal([]objects.Object{objects.NewStr("a"), objects.NewInt(-1)}, nil, nil)
	if err != nil {
		t.Fatalf("decimal('a', -1): %v", err)
	}
	if objects.Repr(got) != "-1" {
		t.Errorf("decimal('a', -1) = %s, want -1", objects.Repr(got))
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
