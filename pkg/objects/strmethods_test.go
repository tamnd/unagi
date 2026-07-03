package objects

import "testing"

// strMCase drives one str method call. Every want and wantErr value in
// this file was probed on CPython 3.14; the comments call out the less
// obvious probes and the deliberate divergences.
type strMCase struct {
	name    string
	s       string
	method  string
	args    []Object
	want    string // Repr of the result
	wantErr string
}

func runStrMCases(t *testing.T, cases []strMCase) {
	t.Helper()
	for _, tt := range cases {
		got, err := CallMethod(NewStr(tt.s), tt.method, tt.args)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		checkRepr(t, tt.name, got, err, tt.want)
	}
}

func TestStrStrip(t *testing.T) {
	runStrMCases(t, []strMCase{
		{"strip default", "  hi \n", "strip", nil, "'hi'", ""},
		{"strip cutset", "xyxhixyx", "strip", []Object{NewStr("xy")}, "'hi'", ""},
		{"strip None", "  hi ", "strip", []Object{None}, "'hi'", ""},
		// Probed on 3.14: \x1c is Python whitespace though not Unicode
		// White_Space.
		{"strip fs", "\x1ca\x1c", "strip", nil, "'a'", ""},
		{"strip empty", "", "strip", nil, "''", ""},
		{"lstrip default", "  hi", "lstrip", nil, "'hi'", ""},
		{"lstrip cutset", "xxhixx", "lstrip", []Object{NewStr("x")}, "'hixx'", ""},
		{"rstrip default", "hi  ", "rstrip", nil, "'hi'", ""},
		{"rstrip cutset", "xxhixx", "rstrip", []Object{NewStr("x")}, "'xxhi'", ""},
		{"strip int", " hi ", "strip", []Object{NewInt(1)}, "", "TypeError: strip arg must be None or str"},
		{"lstrip int", " hi ", "lstrip", []Object{NewInt(1)}, "", "TypeError: lstrip arg must be None or str"},
		{"rstrip int", " hi ", "rstrip", []Object{NewInt(1)}, "", "TypeError: rstrip arg must be None or str"},
		{"strip arity", "a", "strip", []Object{NewStr("a"), NewStr("b")}, "", "TypeError: strip expected at most 1 argument, got 2"},
		{"lstrip arity", "a", "lstrip", []Object{NewStr("a"), NewStr("b")}, "", "TypeError: lstrip expected at most 1 argument, got 2"},
		{"rstrip arity", "a", "rstrip", []Object{NewStr("a"), NewStr("b")}, "", "TypeError: rstrip expected at most 1 argument, got 2"},
	})
}

func TestStrSplit(t *testing.T) {
	runStrMCases(t, []strMCase{
		{"split ws", "a b  c", "split", nil, "['a', 'b', 'c']", ""},
		{"split ws None", "a b  c", "split", []Object{None}, "['a', 'b', 'c']", ""},
		// Probed on 3.14: after the split budget runs out the rest keeps
		// its trailing whitespace.
		{"split ws max1", " a b c ", "split", []Object{None, NewInt(1)}, "['a', 'b c ']", ""},
		{"split ws max2", "  a b c  ", "split", []Object{None, NewInt(2)}, "['a', 'b', 'c  ']", ""},
		{"split ws max0", "a b c", "split", []Object{None, NewInt(0)}, "['a b c']", ""},
		{"split ws only", "   ", "split", []Object{None, NewInt(1)}, "[]", ""},
		{"split ws run", "a  b ", "split", []Object{None, NewInt(1)}, "['a', 'b ']", ""},
		{"split fs ws", "a\x1cb", "split", nil, "['a', 'b']", ""},
		{"split nel ws", "a\u0085b", "split", nil, "['a', 'b']", ""},
		{"split sep max", "a,b,c", "split", []Object{NewStr(","), NewInt(1)}, "['a', 'b,c']", ""},
		{"split sep max0", "a,b,c", "split", []Object{NewStr(","), NewInt(0)}, "['a,b,c']", ""},
		{"split sep neg", "a,b,c", "split", []Object{NewStr(","), NewInt(-1)}, "['a', 'b', 'c']", ""},
		{"split sep neg5", "a,b,c", "split", []Object{NewStr(","), NewInt(-5)}, "['a', 'b', 'c']", ""},
		{"split sep bool", "a,b", "split", []Object{NewStr(","), True}, "['a', 'b']", ""},
		{"split empty ws", "", "split", nil, "[]", ""},
		{"split empty sep", "", "split", []Object{NewStr(",")}, "['']", ""},
		{"split int sep", "a,b", "split", []Object{NewInt(1)}, "", "TypeError: must be str or None, not int"},
		{"split str max", "a,b", "split", []Object{NewStr(","), NewStr("x")}, "", "TypeError: 'str' object cannot be interpreted as an integer"},
		{"split float max", "a,b", "split", []Object{NewStr(","), NewFloat(1)}, "", "TypeError: 'float' object cannot be interpreted as an integer"},
		{"split empty separator", "ab", "split", []Object{NewStr(""), NewInt(1)}, "", "ValueError: empty separator"},
		{"split arity", "a,b", "split", []Object{NewStr(","), NewInt(1), NewInt(2)}, "", "TypeError: split() takes at most 2 arguments (3 given)"},
		{"rsplit ws max", " a b c ", "rsplit", []Object{None, NewInt(1)}, "[' a b', 'c']", ""},
		{"rsplit sep max", "a,b,c", "rsplit", []Object{NewStr(","), NewInt(1)}, "['a,b', 'c']", ""},
		{"rsplit default", "a,b,c", "rsplit", nil, "['a,b,c']", ""},
		{"rsplit ws max0", " a b ", "rsplit", []Object{None, NewInt(0)}, "[' a b']", ""},
		{"rsplit X", "aXbXc", "rsplit", []Object{NewStr("X"), NewInt(1)}, "['aXb', 'c']", ""},
		{"rsplit ws", " x ", "rsplit", nil, "['x']", ""},
		{"rsplit int sep", "a,b", "rsplit", []Object{NewInt(1)}, "", "TypeError: must be str or None, not int"},
		{"rsplit arity", "a", "rsplit", []Object{NewStr(","), NewInt(1), NewInt(2)}, "", "TypeError: rsplit() takes at most 2 arguments (3 given)"},
	})
}

func TestStrSplitlines(t *testing.T) {
	runStrMCases(t, []strMCase{
		{"basic", "a\nb\rc\r\nd", "splitlines", nil, "['a', 'b', 'c', 'd']", ""},
		{"keepends", "a\nb\rc\r\nd", "splitlines", []Object{True}, `['a\n', 'b\r', 'c\r\n', 'd']`, ""},
		// Probed on 3.14: the full boundary set is \n \r \r\n \v \f
		// \x1c \x1d \x1e \x85 \u2028 \u2029.
		{"all boundaries", "a\vb\fc\x1cd\x1de\x1ef\u0085g\u2028h\u2029i", "splitlines", nil,
			"['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i']", ""},
		{"vt ff keepends", "a\vb\fc", "splitlines", []Object{True}, `['a\x0b', 'b\x0c', 'c']`, ""},
		{"trailing nl", "abc\n", "splitlines", nil, "['abc']", ""},
		{"trailing nl keep", "abc\n", "splitlines", []Object{True}, `['abc\n']`, ""},
		{"empty", "", "splitlines", nil, "[]", ""},
		{"lone nl", "\n", "splitlines", nil, "['']", ""},
		{"two nl keep", "\n\n", "splitlines", []Object{True}, `['\n', '\n']`, ""},
		{"no break", "abc", "splitlines", nil, "['abc']", ""},
		{"cr cr", "a\r\rb", "splitlines", nil, "['a', '', 'b']", ""},
		// keepends is any object, judged by truth. Probed on 3.14:
		// "a".splitlines("x") keeps ends, splitlines("") does not.
		{"keepends truthy", "a\nb", "splitlines", []Object{NewStr("x")}, `['a\n', 'b']`, ""},
		{"keepends falsy", "a\nb", "splitlines", []Object{NewStr("")}, "['a', 'b']", ""},
		{"keepends int", "a", "splitlines", []Object{NewInt(1)}, "['a']", ""},
		{"arity", "a", "splitlines", []Object{NewInt(1), NewInt(2)}, "", "TypeError: splitlines() takes at most 1 argument (2 given)"},
	})
}

func TestStrStartsEndsWith(t *testing.T) {
	runStrMCases(t, []strMCase{
		{"tuple hit", "hello", "startswith", []Object{T(NewStr("he"), NewStr("xx"))}, "True", ""},
		{"tuple miss", "hello", "startswith", []Object{T(NewStr("xx"))}, "False", ""},
		{"tuple bad elem", "hello", "startswith", []Object{T(NewStr("xx"), NewInt(1))}, "",
			"TypeError: tuple for startswith must only contain str, not int"},
		// Probed on 3.14: a match ahead of a bad element wins.
		{"tuple lazy", "a", "endswith", []Object{T(NewStr("a"), NewInt(1))}, "True", ""},
		{"start", "hello", "startswith", []Object{NewStr("llo"), NewInt(2)}, "True", ""},
		{"start end", "hello", "startswith", []Object{NewStr("llo"), NewInt(2), NewInt(5)}, "True", ""},
		{"end cuts", "hello", "startswith", []Object{NewStr("llo"), NewInt(2), NewInt(4)}, "False", ""},
		{"clamped", "hello", "startswith", []Object{NewStr("he"), NewInt(-100), NewInt(100)}, "True", ""},
		{"neg start", "hello", "startswith", []Object{NewStr("lo"), NewInt(-2)}, "True", ""},
		{"none bounds", "hello", "startswith", []Object{NewStr("h"), None, None}, "True", ""},
		{"empty both", "", "startswith", []Object{NewStr("")}, "True", ""},
		// Probed on 3.14: start past the end fails even for "".
		{"empty past end", "abc", "startswith", []Object{NewStr(""), NewInt(5)}, "False", ""},
		{"empty inverted", "abc", "startswith", []Object{NewStr(""), NewInt(2), NewInt(1)}, "False", ""},
		{"ends neg end", "hello", "endswith", []Object{NewStr("ll"), NewInt(0), NewInt(-1)}, "True", ""},
		{"ends tuple", "hello", "endswith", []Object{T(NewStr("lo"), NewStr("he"))}, "True", ""},
		{"ends inverted", "abc", "endswith", []Object{NewStr(""), NewInt(2), NewInt(1)}, "False", ""},
		{"ends int", "hello", "endswith", []Object{NewInt(1)}, "",
			"TypeError: endswith first arg must be str or a tuple of str, not int"},
		{"bad slice arg", "hello", "startswith", []Object{NewStr("h"), NewStr("a")}, "",
			"TypeError: slice indices must be integers or None or have an __index__ method"},
		{"no args", "hello", "startswith", nil, "", "TypeError: startswith expected at least 1 argument, got 0"},
		{"four args", "hello", "startswith", []Object{NewStr("a"), NewStr("b"), NewInt(1), NewInt(2)}, "",
			"TypeError: startswith expected at most 3 arguments, got 4"},
		{"ends no args", "a", "endswith", nil, "", "TypeError: endswith expected at least 1 argument, got 0"},
		{"ends four args", "a", "endswith", []Object{NewStr("a"), NewInt(1), NewInt(2), NewInt(3)}, "",
			"TypeError: endswith expected at most 3 arguments, got 4"},
	})
}

func TestStrReplace(t *testing.T) {
	runStrMCases(t, []strMCase{
		{"all", "aaa", "replace", []Object{NewStr("a"), NewStr("b")}, "'bbb'", ""},
		{"count 1", "aaa", "replace", []Object{NewStr("a"), NewStr("b"), NewInt(1)}, "'baa'", ""},
		// Probed on 3.14: a negative count means replace all.
		{"count neg", "aaa", "replace", []Object{NewStr("a"), NewStr("b"), NewInt(-1)}, "'bbb'", ""},
		{"count 0", "aaa", "replace", []Object{NewStr("a"), NewStr("b"), NewInt(0)}, "'aaa'", ""},
		{"count bool", "aaa", "replace", []Object{NewStr("a"), NewStr("b"), True}, "'baa'", ""},
		{"empty old", "abc", "replace", []Object{NewStr(""), NewStr("-")}, "'-a-b-c-'", ""},
		{"empty old count", "abc", "replace", []Object{NewStr(""), NewStr("-"), NewInt(2)}, "'-a-bc'", ""},
		{"empty both", "", "replace", []Object{NewStr(""), NewStr("-")}, "'-'", ""},
		{"one arg", "aaa", "replace", []Object{NewStr("a")}, "",
			"TypeError: replace() takes at least 2 positional arguments (1 given)"},
		{"four args", "aaa", "replace", []Object{NewStr("a"), NewStr("b"), NewInt(1), NewInt(2)}, "",
			"TypeError: replace() takes at most 3 arguments (4 given)"},
		{"str count", "aaa", "replace", []Object{NewStr("a"), NewStr("b"), NewStr("c")}, "",
			"TypeError: 'str' object cannot be interpreted as an integer"},
		{"int old", "aaa", "replace", []Object{NewInt(1), NewStr("b")}, "",
			"TypeError: replace() argument 1 must be str, not int"},
		{"int new", "aaa", "replace", []Object{NewStr("a"), NewInt(2)}, "",
			"TypeError: replace() argument 2 must be str, not int"},
	})
}

func TestStrFindIndexCount(t *testing.T) {
	runStrMCases(t, []strMCase{
		{"find start", "hello", "find", []Object{NewStr("l"), NewInt(3)}, "3", ""},
		{"find window", "hello", "find", []Object{NewStr("l"), NewInt(0), NewInt(2)}, "-1", ""},
		{"find neg start", "hello", "find", []Object{NewStr("l"), NewInt(-2)}, "3", ""},
		{"find none bounds", "hello", "find", []Object{NewStr("l"), None, None}, "2", ""},
		{"find bool start", "hello", "find", []Object{NewStr("l"), True}, "2", ""},
		// Probed on 3.14: the empty needle matches at start while start
		// stays inside the string.
		{"find empty past", "hello", "find", []Object{NewStr(""), NewInt(10)}, "-1", ""},
		{"find empty at", "hello", "find", []Object{NewStr(""), NewInt(4)}, "4", ""},
		{"find empty clamp", "abc", "find", []Object{NewStr(""), NewInt(-100)}, "0", ""},
		{"rfind", "hello", "rfind", []Object{NewStr("l")}, "3", ""},
		{"rfind window", "hello", "rfind", []Object{NewStr("l"), NewInt(0), NewInt(3)}, "2", ""},
		{"rfind unicode", "héllo", "rfind", []Object{NewStr("l")}, "3", ""},
		{"rfind empty at", "hello", "rfind", []Object{NewStr(""), NewInt(5)}, "5", ""},
		{"rfind empty past", "hello", "rfind", []Object{NewStr(""), NewInt(6)}, "-1", ""},
		{"rfind empty end", "abc", "rfind", []Object{NewStr(""), NewInt(0), NewInt(2)}, "2", ""},
		{"index hit", "hello", "index", []Object{NewStr("l"), NewInt(3)}, "3", ""},
		{"index miss", "hello", "index", []Object{NewStr("z")}, "", "ValueError: substring not found"},
		{"rindex hit", "hello", "rindex", []Object{NewStr("l")}, "3", ""},
		{"rindex miss", "hello", "rindex", []Object{NewStr("z")}, "", "ValueError: substring not found"},
		{"index unicode miss", "héllo", "index", []Object{NewStr("z")}, "", "ValueError: substring not found"},
		{"find no args", "hello", "find", nil, "", "TypeError: find expected at least 1 argument, got 0"},
		{"find four args", "hello", "find", []Object{NewStr("a"), NewInt(1), NewInt(2), NewInt(3)}, "",
			"TypeError: find expected at most 3 arguments, got 4"},
		{"find bad start", "hello", "find", []Object{NewStr("a"), NewStr("b")}, "",
			"TypeError: slice indices must be integers or None or have an __index__ method"},
		{"find float start", "hello", "find", []Object{NewStr("l"), NewFloat(1.5)}, "",
			"TypeError: slice indices must be integers or None or have an __index__ method"},
		{"find int sub", "hello", "find", []Object{NewInt(1)}, "", "TypeError: find() argument 1 must be str, not int"},
		{"rfind int sub", "hello", "rfind", []Object{NewInt(1)}, "", "TypeError: rfind() argument 1 must be str, not int"},
		{"rfind no args", "hello", "rfind", nil, "", "TypeError: rfind expected at least 1 argument, got 0"},
		{"index int sub", "hello", "index", []Object{NewInt(1)}, "", "TypeError: index() argument 1 must be str, not int"},
		{"index no args", "hello", "index", nil, "", "TypeError: index expected at least 1 argument, got 0"},
		{"rindex four args", "hello", "rindex", []Object{NewStr("l"), NewInt(1), NewInt(2), NewInt(3)}, "",
			"TypeError: rindex expected at most 3 arguments, got 4"},
		{"count overlap", "aaaa", "count", []Object{NewStr("aa")}, "2", ""},
		{"count start", "aaaa", "count", []Object{NewStr("a"), NewInt(1)}, "3", ""},
		{"count window", "aaaa", "count", []Object{NewStr("a"), NewInt(1), NewInt(3)}, "2", ""},
		{"count none start", "aaaa", "count", []Object{NewStr("a"), None, NewInt(2)}, "2", ""},
		// Probed on 3.14: "abc".count("") is 4, one per gap.
		{"count empty", "abc", "count", []Object{NewStr("")}, "4", ""},
		{"count empty window", "abc", "count", []Object{NewStr(""), NewInt(1), NewInt(2)}, "2", ""},
		{"count empty past", "abc", "count", []Object{NewStr(""), NewInt(5)}, "0", ""},
		{"count empty inverted", "abc", "count", []Object{NewStr(""), NewInt(3), NewInt(1)}, "0", ""},
		{"count inverted", "abc", "count", []Object{NewStr("a"), NewInt(10), NewInt(2)}, "0", ""},
		{"count int sub", "abc", "count", []Object{NewInt(1)}, "", "TypeError: count() argument 1 must be str, not int"},
		{"count no args", "abc", "count", nil, "", "TypeError: count expected at least 1 argument, got 0"},
		{"count bad start", "abc", "count", []Object{NewStr("a"), NewStr("b")}, "",
			"TypeError: slice indices must be integers or None or have an __index__ method"},
	})
}

func TestStrCaseFamily(t *testing.T) {
	runStrMCases(t, []strMCase{
		// Probed on 3.14: capitalize titlecases the first char and
		// lowercases everything after it.
		{"capitalize", "hello world", "capitalize", nil, "'Hello world'", ""},
		{"capitalize downs rest", "HELLO World", "capitalize", nil, "'Hello world'", ""},
		{"capitalize empty", "", "capitalize", nil, "''", ""},
		{"capitalize digit", "1abc", "capitalize", nil, "'1abc'", ""},
		// dz digraph titlecases to the Dz form.
		{"capitalize digraph", "ǆab", "capitalize", nil, "'ǅab'", ""},
		{"title", "hello world", "title", nil, "'Hello World'", ""},
		// Probed on 3.14: the apostrophe is uncased, so the s restarts a
		// word.
		{"title apostrophe", "it's a test", "title", nil, `"It'S A Test"`, ""},
		{"title downs", "HELLO", "title", nil, "'Hello'", ""},
		{"title after digit", "3g ab", "title", nil, "'3G Ab'", ""},
		{"title digraph", "ǆa", "title", nil, "'ǅa'", ""},
		{"title digraph pair", "ǳǳ", "title", nil, "'ǲǳ'", ""},
		{"title keeps Lt", "ǅb", "title", nil, "'ǅb'", ""},
		{"swapcase", "AbC", "swapcase", nil, "'aBc'", ""},
		{"swapcase words", "fOO BAR", "swapcase", nil, "'Foo bar'", ""},
		// micro sign upcases to Greek capital mu; a titlecase char stays.
		{"swapcase micro", "µ", "swapcase", nil, "'Μ'", ""},
		{"swapcase Lt", "ǅ", "swapcase", nil, "'ǅ'", ""},
		{"capitalize arity", "a", "capitalize", []Object{NewInt(1)}, "",
			"TypeError: str.capitalize() takes no arguments (1 given)"},
		{"title arity", "a", "title", []Object{NewInt(1)}, "", "TypeError: str.title() takes no arguments (1 given)"},
		{"swapcase arity", "a", "swapcase", []Object{NewInt(1)}, "", "TypeError: str.swapcase() takes no arguments (1 given)"},
	})
}

func TestStrPredicates(t *testing.T) {
	// Empty-string results, all probed on 3.14: only isascii and
	// isprintable are True.
	empties := map[string]string{
		"isalnum": "False", "isalpha": "False", "isascii": "True",
		"isdecimal": "False", "isdigit": "False", "isidentifier": "False",
		"islower": "False", "isnumeric": "False", "isprintable": "True",
		"isspace": "False", "istitle": "False", "isupper": "False",
	}
	var cases []strMCase
	for m, want := range empties {
		cases = append(cases, strMCase{"empty " + m, "", m, nil, want, ""})
		cases = append(cases, strMCase{m + " arity", "a", m, []Object{NewInt(1)}, "",
			"TypeError: str." + m + "() takes no arguments (1 given)"})
	}
	// The decimal/digit/numeric ladder, probed on 3.14: ASCII five is
	// all three, superscript two drops isdecimal, the vulgar fraction
	// and the Roman numeral and the Han numeral keep only isnumeric.
	ladder := []struct {
		s                       string
		decimal, digit, numeric string
	}{
		{"5", "True", "True", "True"},
		{"²", "False", "True", "True"},
		{"½", "False", "False", "True"},
		{"Ⅸ", "False", "False", "True"},
		{"一", "False", "False", "True"},
		{"a5", "False", "False", "False"},
	}
	for _, l := range ladder {
		cases = append(cases,
			strMCase{l.s + " isdecimal", l.s, "isdecimal", nil, l.decimal, ""},
			strMCase{l.s + " isdigit", l.s, "isdigit", nil, l.digit, ""},
			strMCase{l.s + " isnumeric", l.s, "isnumeric", nil, l.numeric, ""},
		)
	}
	cases = append(cases, []strMCase{
		{"alnum letters digits", "ab2", "isalnum", nil, "True", ""},
		{"alnum numeric", "½", "isalnum", nil, "True", ""},
		{"alnum roman", "Ⅸ", "isalnum", nil, "True", ""},
		{"alnum space", "a 5", "isalnum", nil, "False", ""},
		{"alpha unicode", "héllo", "isalpha", nil, "True", ""},
		{"alpha space", "ab c", "isalpha", nil, "False", ""},
		{"alpha superscript", "²", "isalpha", nil, "False", ""},
		{"ascii yes", "hello\x7f", "isascii", nil, "True", ""},
		{"ascii no", "héllo", "isascii", nil, "False", ""},
		{"lower plain", "abc", "islower", nil, "True", ""},
		// Uncased chars are ignored as long as one cased char is lower.
		{"lower with digit", "ab1", "islower", nil, "True", ""},
		{"lower uncased only", "123", "islower", nil, "False", ""},
		{"lower mixed", "aBc", "islower", nil, "False", ""},
		{"lower sharp s", "ß", "islower", nil, "True", ""},
		{"lower ligature", "ﬁ", "islower", nil, "True", ""},
		{"upper plain", "ABC", "isupper", nil, "True", ""},
		{"upper with digit", "AB1", "isupper", nil, "True", ""},
		{"upper uncased only", "123", "isupper", nil, "False", ""},
		// The Dz titlecase char is neither upper nor lower but is title.
		{"upper Lt", "ǅ", "isupper", nil, "False", ""},
		{"lower Lt", "ǅ", "islower", nil, "False", ""},
		{"title Lt", "ǅ", "istitle", nil, "True", ""},
		{"title basic", "Hello World", "istitle", nil, "True", ""},
		{"title lower word", "Hello world", "istitle", nil, "False", ""},
		{"title caps", "HELLO", "istitle", nil, "False", ""},
		{"title double space", "Hello  World", "istitle", nil, "True", ""},
		{"title apostrophe", "It'S", "istitle", nil, "True", ""},
		{"title apostrophe low", "It's", "istitle", nil, "False", ""},
		{"title single", "A", "istitle", nil, "True", ""},
		{"title digits around", "1A1", "istitle", nil, "True", ""},
		{"title digit low", "1a", "istitle", nil, "False", ""},
		{"title space", " ", "istitle", nil, "False", ""},
		{"title sharp s", "ß", "istitle", nil, "False", ""},
		{"space plain", " \t\n", "isspace", nil, "True", ""},
		{"space fs", "\x1c", "isspace", nil, "True", ""},
		{"space nbsp", "\u00a0", "isspace", nil, "True", ""},
		{"space zwsp", "\u200b", "isspace", nil, "False", ""},
		{"printable plain", "abc def", "isprintable", nil, "True", ""},
		{"printable nl", "ab\n", "isprintable", nil, "False", ""},
		{"printable del", "\x7f", "isprintable", nil, "False", ""},
		{"printable nbsp", "\u00a0", "isprintable", nil, "False", ""},
		{"ident plain", "_abc1", "isidentifier", nil, "True", ""},
		{"ident digit start", "1abc", "isidentifier", nil, "False", ""},
		{"ident keyword", "for", "isidentifier", nil, "True", ""},
		{"ident dash", "abc-def", "isidentifier", nil, "False", ""},
		{"ident lambda", "λ", "isidentifier", nil, "True", ""},
	}...)
	runStrMCases(t, cases)
}

func TestStrJustify(t *testing.T) {
	runStrMCases(t, []strMCase{
		// Probed on 3.14: the odd margin char of center goes by
		// marg/2 + (marg & width & 1).
		{"center even", "abc", "center", []Object{NewInt(6)}, "' abc  '", ""},
		{"center odd", "abc", "center", []Object{NewInt(7)}, "'  abc  '", ""},
		{"center odd width", "ab", "center", []Object{NewInt(5)}, "'  ab '", ""},
		{"center fill", "ab", "center", []Object{NewInt(5), NewStr("*")}, "'**ab*'", ""},
		{"center short", "abc", "center", []Object{NewInt(2)}, "'abc'", ""},
		{"center neg", "abc", "center", []Object{NewInt(-1)}, "'abc'", ""},
		{"center bool width", "abc", "center", []Object{True}, "'abc'", ""},
		{"ljust", "abc", "ljust", []Object{NewInt(6), NewStr("-")}, "'abc---'", ""},
		{"ljust short", "abc", "ljust", []Object{NewInt(2)}, "'abc'", ""},
		{"ljust unicode fill", "abc", "ljust", []Object{NewInt(5), NewStr("é")}, "'abcéé'", ""},
		{"rjust", "abc", "rjust", []Object{NewInt(6), NewStr("-")}, "'---abc'", ""},
		{"rjust bool", "abc", "rjust", []Object{True}, "'abc'", ""},
		{"center two chars", "abc", "center", []Object{NewInt(6), NewStr("ab")}, "",
			"TypeError: The fill character must be exactly one character long"},
		{"ljust two chars", "abc", "ljust", []Object{NewInt(6), NewStr("ab")}, "",
			"TypeError: The fill character must be exactly one character long"},
		{"rjust empty fill", "abc", "rjust", []Object{NewInt(6), NewStr("")}, "",
			"TypeError: The fill character must be exactly one character long"},
		{"center int fill", "abc", "center", []Object{NewInt(6), NewInt(1)}, "",
			"TypeError: The fill character must be a unicode character, not int"},
		{"center no args", "abc", "center", nil, "", "TypeError: center expected at least 1 argument, got 0"},
		{"center str width", "abc", "center", []Object{NewStr("a")}, "",
			"TypeError: 'str' object cannot be interpreted as an integer"},
		{"center three args", "abc", "center", []Object{NewInt(5), NewStr("x"), NewStr("y")}, "",
			"TypeError: center expected at most 2 arguments, got 3"},
		{"ljust no args", "a", "ljust", nil, "", "TypeError: ljust expected at least 1 argument, got 0"},
		{"ljust three args", "a", "ljust", []Object{NewInt(1), NewStr("x"), NewStr("y")}, "",
			"TypeError: ljust expected at most 2 arguments, got 3"},
		{"rjust no args", "a", "rjust", nil, "", "TypeError: rjust expected at least 1 argument, got 0"},
		{"rjust three args", "a", "rjust", []Object{NewInt(1), NewStr("x"), NewStr("y")}, "",
			"TypeError: rjust expected at most 2 arguments, got 3"},
	})
}

func TestStrZfill(t *testing.T) {
	runStrMCases(t, []strMCase{
		{"plain", "12", "zfill", []Object{NewInt(5)}, "'00012'", ""},
		// Probed on 3.14: an ASCII sign stays in front of the zeros.
		{"plus", "+12", "zfill", []Object{NewInt(5)}, "'+0012'", ""},
		{"minus", "-12", "zfill", []Object{NewInt(5)}, "'-0012'", ""},
		{"non numeric", "ab", "zfill", []Object{NewInt(5)}, "'000ab'", ""},
		{"sign only", "+", "zfill", []Object{NewInt(3)}, "'+00'", ""},
		{"empty", "", "zfill", []Object{NewInt(3)}, "'000'", ""},
		{"width equal", "12", "zfill", []Object{NewInt(2)}, "'12'", ""},
		{"width short", "12", "zfill", []Object{NewInt(1)}, "'12'", ""},
		{"minus short", "-", "zfill", []Object{NewInt(1)}, "'-'", ""},
		// The Unicode minus sign is not a sign to zfill.
		{"unicode minus", "−12", "zfill", []Object{NewInt(5)}, "'00−12'", ""},
		{"no args", "12", "zfill", nil, "", "TypeError: str.zfill() takes exactly one argument (0 given)"},
		{"str width", "12", "zfill", []Object{NewStr("a")}, "",
			"TypeError: 'str' object cannot be interpreted as an integer"},
		{"two args", "12", "zfill", []Object{NewInt(3), NewInt(4)}, "",
			"TypeError: str.zfill() takes exactly one argument (2 given)"},
	})
}

func TestStrExpandtabs(t *testing.T) {
	runStrMCases(t, []strMCase{
		{"default 8", "a\tb", "expandtabs", nil, "'a       b'", ""},
		{"size 4", "a\tb", "expandtabs", []Object{NewInt(4)}, "'a   b'", ""},
		// Probed on 3.14: non-positive sizes just delete tabs.
		{"size 0", "a\tb", "expandtabs", []Object{NewInt(0)}, "'ab'", ""},
		{"size neg", "a\tb", "expandtabs", []Object{NewInt(-1)}, "'ab'", ""},
		{"lone tab", "\t", "expandtabs", []Object{NewInt(4)}, "'    '", ""},
		// Newlines and carriage returns reset the column.
		{"newline reset", "ab\tc\nd\te", "expandtabs", []Object{NewInt(4)}, `'ab  c\nd   e'`, ""},
		{"cr reset", "ab\rcd\tx", "expandtabs", []Object{NewInt(4)}, `'ab\rcd  x'`, ""},
		{"code point cols", "héé\tb", "expandtabs", []Object{NewInt(4)}, "'héé b'", ""},
		{"wide cols", "日本\t語", "expandtabs", []Object{NewInt(4)}, "'日本  語'", ""},
		{"double tab", "a\t\tb", "expandtabs", []Object{NewInt(4)}, "'a       b'", ""},
		{"str size", "a\tb", "expandtabs", []Object{NewStr("x")}, "",
			"TypeError: 'str' object cannot be interpreted as an integer"},
		{"two args", "a\tb", "expandtabs", []Object{NewInt(4), NewInt(5)}, "",
			"TypeError: expandtabs() takes at most 1 argument (2 given)"},
	})
}

func TestStrPartition(t *testing.T) {
	runStrMCases(t, []strMCase{
		{"hit", "a,b,c", "partition", []Object{NewStr(",")}, "('a', ',', 'b,c')", ""},
		{"miss", "abc", "partition", []Object{NewStr(",")}, "('abc', '', '')", ""},
		{"rhit", "a,b,c", "rpartition", []Object{NewStr(",")}, "('a,b', ',', 'c')", ""},
		{"rmiss", "abc", "rpartition", []Object{NewStr(",")}, "('', '', 'abc')", ""},
		{"unicode", "héllo", "partition", []Object{NewStr("l")}, "('hé', 'l', 'lo')", ""},
		{"empty sep", "abc", "partition", []Object{NewStr("")}, "", "ValueError: empty separator"},
		{"rempty sep", "abc", "rpartition", []Object{NewStr("")}, "", "ValueError: empty separator"},
		{"int sep", "abc", "partition", []Object{NewInt(1)}, "", "TypeError: must be str, not int"},
		{"rint sep", "abc", "rpartition", []Object{NewInt(1)}, "", "TypeError: must be str, not int"},
		{"no args", "abc", "partition", nil, "", "TypeError: str.partition() takes exactly one argument (0 given)"},
		{"two args", "abc", "partition", []Object{NewStr("a"), NewStr("b")}, "",
			"TypeError: str.partition() takes exactly one argument (2 given)"},
		{"rno args", "a", "rpartition", nil, "", "TypeError: str.rpartition() takes exactly one argument (0 given)"},
		{"rtwo args", "a", "rpartition", []Object{NewStr("a"), NewStr("b")}, "",
			"TypeError: str.rpartition() takes exactly one argument (2 given)"},
	})
}

func TestStrRemovePrefixSuffix(t *testing.T) {
	runStrMCases(t, []strMCase{
		{"prefix hit", "TestHook", "removeprefix", []Object{NewStr("Test")}, "'Hook'", ""},
		{"prefix miss", "TestHook", "removeprefix", []Object{NewStr("X")}, "'TestHook'", ""},
		{"prefix empty", "abc", "removeprefix", []Object{NewStr("")}, "'abc'", ""},
		{"prefix all", "abc", "removeprefix", []Object{NewStr("abc")}, "''", ""},
		{"suffix hit", "MiscTests", "removesuffix", []Object{NewStr("Tests")}, "'Misc'", ""},
		{"suffix miss", "MiscTests", "removesuffix", []Object{NewStr("X")}, "'MiscTests'", ""},
		// Only one copy comes off.
		{"suffix once", "abab", "removesuffix", []Object{NewStr("ab")}, "'ab'", ""},
		{"prefix int", "abc", "removeprefix", []Object{NewInt(1)}, "",
			"TypeError: removeprefix() argument must be str, not int"},
		{"suffix int", "abc", "removesuffix", []Object{NewInt(1)}, "",
			"TypeError: removesuffix() argument must be str, not int"},
		{"prefix no args", "abc", "removeprefix", nil, "",
			"TypeError: str.removeprefix() takes exactly one argument (0 given)"},
		{"prefix two args", "abc", "removeprefix", []Object{NewStr("a"), NewStr("b")}, "",
			"TypeError: str.removeprefix() takes exactly one argument (2 given)"},
		{"suffix no args", "a", "removesuffix", nil, "",
			"TypeError: str.removesuffix() takes exactly one argument (0 given)"},
		{"suffix two args", "a", "removesuffix", []Object{NewStr("a"), NewStr("b")}, "",
			"TypeError: str.removesuffix() takes exactly one argument (2 given)"},
	})
}
