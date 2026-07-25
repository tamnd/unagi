package runtime

import (
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
		{"\x00", "Cc"}, {"​", "Cf"}, {"\U000f0000", "Co"},
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

// TestUnicodedataNormalize checks the ASCII fast path returns the string
// unchanged, an invalid form is a ValueError, and non-ASCII text is refused
// rather than silently wrong.
func TestUnicodedataNormalize(t *testing.T) {
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
	if _, err := udNormalize([]objects.Object{objects.NewStr("NFC"), objects.NewStr("é")}); err == nil {
		t.Errorf("normalize on non-ASCII should raise rather than return wrong text")
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
