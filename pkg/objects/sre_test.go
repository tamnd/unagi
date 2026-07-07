package objects

import "testing"

func TestPatternAttrs(t *testing.T) {
	gi, err := NewDict([]Object{NewStr("x")}, []Object{NewInt(1)})
	if err != nil {
		t.Fatal(err)
	}
	ig := NewTuple([]Object{None, NewStr("x")})
	p := NewPattern(NewStr("(?P<x>a)"), SreFlagUnicode, []uint32{16, 97, 1}, 1, gi, ig, false)

	if v, err := patternAttr(p.(*patternObject), "pattern"); err != nil || Str(v) != "(?P<x>a)" {
		t.Errorf("pattern = %v, %v", v, err)
	}
	if v, err := patternAttr(p.(*patternObject), "flags"); err != nil {
		t.Fatal(err)
	} else if n, _ := AsInt(v); n != int64(SreFlagUnicode) {
		t.Errorf("flags = %d, want %d", n, SreFlagUnicode)
	}
	if v, err := patternAttr(p.(*patternObject), "groups"); err != nil {
		t.Fatal(err)
	} else if n, _ := AsInt(v); n != 1 {
		t.Errorf("groups = %d, want 1", n)
	}
	if _, err := patternAttr(p.(*patternObject), "bogus"); err == nil {
		t.Error("bogus attribute should raise")
	}
}

func TestPatternFlagRepr(t *testing.T) {
	cases := []struct {
		flags   uint32
		isbytes bool
		want    string
	}{
		{0, false, ""},
		{SreFlagUnicode, false, ""},          // implied default for str, dropped
		{SreFlagUnicode, true, "re.UNICODE"}, // shown for bytes
		{SreFlagIgnorecase, false, "re.IGNORECASE"},
		{SreFlagIgnorecase | SreFlagMultiline, false, "re.IGNORECASE|re.MULTILINE"},
		{SreFlagUnicode | SreFlagAscii, false, "re.UNICODE|re.ASCII"}, // locale or ascii present, keep unicode
		{512, false, "0x200"}, // unknown bit as hex
	}
	for _, c := range cases {
		if got := patternFlagRepr(c.flags, c.isbytes); got != c.want {
			t.Errorf("patternFlagRepr(%d, %v) = %q, want %q", c.flags, c.isbytes, got, c.want)
		}
	}
}

func TestPatternRepr(t *testing.T) {
	p := NewPattern(NewStr("a"), SreFlagIgnorecase, []uint32{16, 97, 1}, 0, None, NewTuple(nil), false)
	got, err := patternRepr(p.(*patternObject), true)
	if err != nil {
		t.Fatal(err)
	}
	if got != "re.compile('a', re.IGNORECASE)" {
		t.Errorf("repr = %q", got)
	}
}
