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

// TestStrCasefold covers the objects-side casefold logic: the full-fold table is
// reached through CaseFoldHook, a code point absent from it folds to itself, a
// lone surrogate passes through, and with no hook (unicodedata not linked) the
// fold falls back to the simple lowercase.
func TestStrCasefold(t *testing.T) {
	// A stub fold table: the German sharp s expands to ss, A folds to a, and
	// nothing else has an entry so it folds to itself.
	CaseFoldHook = func(r rune) []rune {
		switch r {
		case 0x00DF: // ß
			return []rune{'s', 's'}
		case 'A':
			return []rune{'a'}
		}
		return nil
	}
	defer func() { CaseFoldHook = nil }()

	runStrMCases(t, []strMCase{
		{"fold expand", "Aß", "casefold", nil, "'ass'", ""},
		{"fold identity", "hello", "casefold", nil, "'hello'", ""},
		{"fold arity", "a", "casefold", []Object{NewInt(1)}, "",
			"TypeError: str.casefold() takes no arguments (1 given)"},
	})

	// A lone surrogate is not in the table, so it survives in its WTF-8 form.
	sur := StrFromRune(0xDC80)
	got := strCasefold(sur)
	if got != sur {
		t.Fatalf("casefold surrogate = %q, want %q", got, sur)
	}

	// With no hook the fold degrades to simple lowercase, so ß stays ß rather
	// than expanding, but an ASCII letter still lowercases.
	CaseFoldHook = nil
	if got := strCasefold("AbßC"); got != "abßc" {
		t.Fatalf("casefold fallback = %q, want %q", got, "abßc")
	}
}

// TestStrUpperFull covers the objects-side upper logic: the full-uppercase table
// is reached through UpperFullHook, a code point absent from it uppercases to
// itself, a lone surrogate passes through, and with no hook (unicodedata not
// linked) upper falls back to Go's simple uppercase.
func TestStrUpperFull(t *testing.T) {
	// A stub table: the German sharp s expands to SS and a lowercase a maps to A,
	// nothing else has an entry so it uppercases to itself.
	UpperFullHook = func(r rune) []rune {
		switch r {
		case 0x00DF: // ß
			return []rune{'S', 'S'}
		case 'a':
			return []rune{'A'}
		}
		return nil
	}
	defer func() { UpperFullHook = nil }()

	runStrMCases(t, []strMCase{
		{"upper expand", "aß", "upper", nil, "'ASS'", ""},
		{"upper identity", "XYZ", "upper", nil, "'XYZ'", ""},
		{"upper arity", "a", "upper", []Object{NewInt(1)}, "",
			"TypeError: str.upper() takes no arguments (1 given)"},
	})

	// A lone surrogate is not in the table, so it survives in its WTF-8 form.
	sur := StrFromRune(0xDC80)
	if got := strUpper(sur); got != sur {
		t.Fatalf("upper surrogate = %q, want %q", got, sur)
	}

	// With no hook upper degrades to Go's simple uppercase, so ß stays ß rather
	// than expanding, but an ASCII letter still uppercases.
	UpperFullHook = nil
	if got := strUpper("aßb"); got != "AßB" {
		t.Fatalf("upper fallback = %q, want %q", got, "AßB")
	}
}

// TestStrLowerFull covers the objects-side lower logic: the full-lowercase table
// is reached through LowerFullHook, the Greek capital sigma is not in the table
// but takes its final or plain form from the Final_Sigma walk over the Cased and
// Case_Ignorable hooks, a lone surrogate passes through, and with no hook
// (unicodedata not linked) lower falls back to Go's simple lowercase.
func TestStrLowerFull(t *testing.T) {
	// A stub table: an uppercase A lowercases to a, the Turkish dotted capital I
	// expands to i plus a combining dot, nothing else has an entry. The sigma is
	// left out on purpose, the walk handles it.
	LowerFullHook = func(r rune) []rune {
		switch r {
		case 'A':
			return []rune{'a'}
		case 0x0130: // İ
			return []rune{'i', 0x0307}
		case 0x039F: // Ο
			return []rune{0x03BF}
		}
		return nil
	}
	// Latin A and the Greek capital letters used below are cased; the combining
	// dot above is case-ignorable and transparent to the walk.
	CasedHook = func(r rune) bool {
		return r == 'A' || (r >= 0x0391 && r <= 0x03A9) || (r >= 0x03B1 && r <= 0x03C9)
	}
	CaseIgnorableHook = func(r rune) bool { return r == 0x0307 }
	defer func() {
		LowerFullHook = nil
		CasedHook = nil
		CaseIgnorableHook = nil
	}()

	runStrMCases(t, []strMCase{
		{"lower table", "AA", "lower", nil, "'aa'", ""},
		{"lower expand", "İ", "lower", nil, "'i̇'", ""},
		// Σ preceded by a cased Ο and ending the word takes the final form ς.
		{"sigma final", "ΟΣ", "lower", nil, "'ος'", ""},
		// Σ between two cased letters takes the plain form σ.
		{"sigma medial", "ΟΣΟ", "lower", nil, "'οσο'", ""},
		// A leading Σ has no preceding cased letter, so it is not final.
		{"sigma initial", "ΣΟ", "lower", nil, "'σο'", ""},
		// A case-ignorable dot between the cased Ο and the sigma is skipped, so the
		// sigma is still word-final.
		{"sigma skips ignorable", "Ο̇Σ", "lower", nil, "'ο̇ς'", ""},
		{"lower arity", "a", "lower", []Object{NewInt(1)}, "",
			"TypeError: str.lower() takes no arguments (1 given)"},
	})

	// A lone surrogate is not in the table, so it survives in its WTF-8 form.
	sur := StrFromRune(0xDC80)
	if got := strLower(sur); got != sur {
		t.Fatalf("lower surrogate = %q, want %q", got, sur)
	}

	// With no hook lower degrades to Go's simple lowercase, so the sigma folds to
	// the plain form with no context and an ASCII letter still lowercases.
	LowerFullHook = nil
	if got := strLower("AΣB"); got != "aσb" {
		t.Fatalf("lower fallback = %q, want %q", got, "aσb")
	}
}

// TestStrTitleFull covers the objects-side title and capitalize logic: the first
// cased character of each word takes the full titlecase table, the rest lowercase
// in the whole-string context (so a word-final sigma takes its final form), a
// lone surrogate passes through, and with no hook both fall back to Go's simple
// mapping.
func TestStrTitleFull(t *testing.T) {
	// Stub tables: the German sharp s titlecases to Ss, ASCII letters map through
	// their case, and the Greek omicron lowercases to its own lowercase. The sigma
	// is left out of the lower table, the walk handles it.
	TitleFullHook = func(r rune) []rune {
		switch r {
		case 0x00DF: // ß
			return []rune{'S', 's'}
		case 'a':
			return []rune{'A'}
		case 'b':
			return []rune{'B'}
		}
		return nil
	}
	LowerFullHook = func(r rune) []rune {
		switch r {
		case 'A':
			return []rune{'a'}
		case 'B':
			return []rune{'b'}
		case 0x039F: // Ο
			return []rune{0x03BF}
		}
		return nil
	}
	CasedHook = func(r rune) bool {
		return r == 'a' || r == 'b' || r == 'A' || r == 'B' || r == 0x00DF ||
			(r >= 0x0391 && r <= 0x03A9) || (r >= 0x03B1 && r <= 0x03C9)
	}
	CaseIgnorableHook = func(r rune) bool { return r == 0x0307 }
	defer func() {
		TitleFullHook = nil
		LowerFullHook = nil
		CasedHook = nil
		CaseIgnorableHook = nil
	}()

	runStrMCases(t, []strMCase{
		// The word-leading sharp s titlecases to Ss, then the cased tail lowercases.
		{"title expand", "ßb", "title", nil, "'Ssb'", ""},
		// An uncased space starts a new word, so both letters titlecase.
		{"title words", "a b", "title", nil, "'A B'", ""},
		// The tail sigma is preceded by a cased letter and ends the word, so it
		// takes the final form ς.
		{"title sigma", "ΟΣ", "title", nil, "'Ος'", ""},
		{"cap expand", "ßb", "capitalize", nil, "'Ssb'", ""},
		// capitalize lowercases the tail, so the trailing sigma is word-final.
		{"cap sigma", "ΟΣ", "capitalize", nil, "'Ος'", ""},
		{"title arity", "a", "title", []Object{NewInt(1)}, "",
			"TypeError: str.title() takes no arguments (1 given)"},
	})

	// A lone surrogate is in neither table, so it survives in its WTF-8 form and
	// does not count as a cased letter.
	sur := StrFromRune(0xDC80)
	if got := strTitle(sur); got != sur {
		t.Fatalf("title surrogate = %q, want %q", got, sur)
	}
	if got := strCapitalize(sur); got != sur {
		t.Fatalf("capitalize surrogate = %q, want %q", got, sur)
	}

	// With no hook both degrade to Go's simple titlecase and lowercase, so the
	// sharp s stays a single character rather than expanding.
	TitleFullHook = nil
	LowerFullHook = nil
	if got := strTitle("ab cd"); got != "Ab Cd" {
		t.Fatalf("title fallback = %q, want %q", got, "Ab Cd")
	}
	if got := strCapitalize("aB"); got != "Ab" {
		t.Fatalf("capitalize fallback = %q, want %q", got, "Ab")
	}
}

// TestStrSwapcaseFull covers the objects-side swapcase logic: an uppercase
// character lowercases in the whole-string context (so a word-final sigma takes
// its final form), a lowercase one takes the full uppercase, a titlecase or
// caseless character is left alone, a lone surrogate passes through, and with no
// hook it falls back to Go's simple properties.
func TestStrSwapcaseFull(t *testing.T) {
	// Stub tables and branch sets: the German sharp s (lowercase) uppercases to
	// SS, ASCII letters and the Greek omicron swap through their case, and the
	// titlecase digraph U+01C5 is in neither branch set so it is left alone.
	UpperFullHook = func(r rune) []rune {
		switch r {
		case 0x00DF: // ß
			return []rune{'S', 'S'}
		case 'a':
			return []rune{'A'}
		case 0x03BF: // ο
			return []rune{0x039F}
		}
		return nil
	}
	LowerFullHook = func(r rune) []rune {
		switch r {
		case 'A':
			return []rune{'a'}
		case 0x039F: // Ο
			return []rune{0x03BF}
		}
		return nil
	}
	UppercaseHook = func(r rune) bool {
		return r == 'A' || r == 0x03A3 || (r >= 0x0391 && r <= 0x03A9)
	}
	LowercaseHook = func(r rune) bool {
		return r == 'a' || r == 0x00DF || (r >= 0x03B1 && r <= 0x03C9)
	}
	CasedHook = func(r rune) bool {
		return r == 'a' || r == 'A' || r == 0x00DF ||
			(r >= 0x0391 && r <= 0x03A9) || (r >= 0x03B1 && r <= 0x03C9)
	}
	CaseIgnorableHook = func(r rune) bool { return r == 0x0307 }
	defer func() {
		UpperFullHook = nil
		LowerFullHook = nil
		UppercaseHook = nil
		LowercaseHook = nil
		CasedHook = nil
		CaseIgnorableHook = nil
	}()

	runStrMCases(t, []strMCase{
		// The lowercase sharp s uppercases to SS, the uppercase A lowercases.
		{"swap expand", "Aß", "swapcase", nil, "'aSS'", ""},
		// The titlecase digraph is in neither branch, so it is left alone.
		{"swap title", "ǅ", "swapcase", nil, "'ǅ'", ""},
		// The uppercase sigma lowercases word-finally, so it takes the final form.
		{"swap sigma final", "ΟΣ", "swapcase", nil, "'ος'", ""},
		// A lone uppercase sigma has no preceding cased letter, so it is not final.
		{"swap sigma initial", "Σο", "swapcase", nil, "'σΟ'", ""},
		{"swap arity", "a", "swapcase", []Object{NewInt(1)}, "",
			"TypeError: str.swapcase() takes no arguments (1 given)"},
	})

	// A lone surrogate is in neither branch set, so it survives in its WTF-8 form.
	sur := StrFromRune(0xDC80)
	if got := strSwapcase(sur); got != sur {
		t.Fatalf("swapcase surrogate = %q, want %q", got, sur)
	}

	// With no hook swapcase degrades to Go's simple properties, so the sharp s
	// stays a single character rather than expanding.
	UppercaseHook = nil
	LowercaseHook = nil
	if got := strSwapcase("aB"); got != "Ab" {
		t.Fatalf("swapcase fallback = %q, want %q", got, "Ab")
	}
}

// TestStrCasePredicatesProp covers str.isupper / str.islower routing through the
// pinned Uppercase and Lowercase property hooks. The stub sets add a newer
// bicameral pair Go's older tables miss (the Garay capital and small A) and a
// caseless-cased mathematical capital that keeps the Uppercase property while
// mapping to itself, so the predicates agree with CPython where Go's simple
// properties would not.
func TestStrCasePredicatesProp(t *testing.T) {
	// U+10D50 GARAY CAPITAL LETTER A and U+2102 DOUBLE-STRUCK CAPITAL C carry the
	// Uppercase property; U+10D70 GARAY SMALL LETTER A carries the Lowercase one.
	UppercaseHook = func(r rune) bool { return r == 'A' || r == 0x10D50 || r == 0x2102 }
	LowercaseHook = func(r rune) bool { return r == 'a' || r == 0x10D70 }
	defer func() {
		UppercaseHook = nil
		LowercaseHook = nil
	}()

	runStrMCases(t, []strMCase{
		// The Garay capital and the double-struck C are uppercase to the pinned set.
		{"isupper garay", "\U00010D50", "isupper", nil, "True", ""},
		{"isupper mathcap", "ℂ", "isupper", nil, "True", ""},
		{"isupper garay small", "\U00010D70", "isupper", nil, "False", ""},
		// The Garay small letter is lowercase, its capital is not.
		{"islower garay", "\U00010D70", "islower", nil, "True", ""},
		{"islower garay cap", "\U00010D50", "islower", nil, "False", ""},
		// A title of the newer pair follows the Uppercase-then-Lowercase shape.
		{"istitle garay", "\U00010D50\U00010D70", "istitle", nil, "True", ""},
		{"istitle garay two caps", "\U00010D50\U00010D50", "istitle", nil, "False", ""},
	})

	// With no hook the predicates degrade to Go's simple properties, so the newer
	// Garay capital is unknown and isupper reports false while ASCII still works.
	UppercaseHook = nil
	LowercaseHook = nil
	if strPredicate("isupper", "\U00010D50") {
		t.Fatalf("isupper fallback garay = true, want false")
	}
	if !strPredicate("isupper", "ABC") {
		t.Fatalf("isupper fallback ABC = false, want true")
	}
}

// TestStrClassifyPreds covers str.isalpha / isalnum / isdecimal / isdigit /
// isnumeric / isprintable routing through the pinned category and digit/numeric
// value hooks. The stubs add a newer-block letter Go's tables miss (the Garay
// capital A, category Lu), a superscript two that carries a digit and a numeric
// value without being a decimal, and a Roman numeral that is numeric but not a
// letter, so the predicates split the classes the way CPython does.
func TestStrClassifyPreds(t *testing.T) {
	CategoryHook = func(r rune) string {
		switch r {
		case 'A', 0x10D50: // ASCII cap and GARAY CAPITAL LETTER A
			return "Lu"
		case 'a':
			return "Ll"
		case '5':
			return "Nd"
		case 0x00B2: // SUPERSCRIPT TWO
			return "No"
		case 0x2160: // ROMAN NUMERAL ONE
			return "Nl"
		case ' ':
			return "Zs"
		case 0x007F: // DELETE
			return "Cc"
		case 0xDC80: // lone surrogate
			return "Cs"
		case '!':
			return "Po"
		}
		return "Cn"
	}
	DigitHook = func(r rune) bool { return r == '5' || r == 0x00B2 }
	NumericHook = func(r rune) bool { return r == '5' || r == 0x00B2 || r == 0x2160 }
	defer func() {
		CategoryHook = nil
		DigitHook = nil
		NumericHook = nil
	}()

	runStrMCases(t, []strMCase{
		// The Garay capital is a letter to the pinned category, a decimal digit is not.
		{"isalpha garay", "\U00010D50", "isalpha", nil, "True", ""},
		{"isalpha digit", "5", "isalpha", nil, "False", ""},
		{"isalpha mixed", "A\U00010D50", "isalpha", nil, "True", ""},
		// Only the Nd decimal counts as decimal; the superscript and Roman numeral
		// carry a digit or numeric value but are not decimals.
		{"isdecimal five", "5", "isdecimal", nil, "True", ""},
		{"isdecimal super", "²", "isdecimal", nil, "False", ""},
		{"isdigit super", "²", "isdigit", nil, "True", ""},
		{"isdigit roman", "Ⅰ", "isdigit", nil, "False", ""},
		{"isnumeric roman", "Ⅰ", "isnumeric", nil, "True", ""},
		{"isnumeric letter", "A", "isnumeric", nil, "False", ""},
		// isalnum is the union: a letter, a decimal, a digit or a numeric all pass.
		{"isalnum letter", "A", "isalnum", nil, "True", ""},
		{"isalnum roman", "Ⅰ", "isalnum", nil, "True", ""},
		{"isalnum punct", "!", "isalnum", nil, "False", ""},
		// isprintable drops the Other and Separator categories but keeps the space.
		{"isprintable space", " ", "isprintable", nil, "True", ""},
		{"isprintable ctrl", "\x7f", "isprintable", nil, "False", ""},
		{"isprintable punct", "A!", "isprintable", nil, "True", ""},
	})

	// A lone surrogate is category Cs, so it is not printable.
	if strPredicate("isprintable", StrFromRune(0xDC80)) {
		t.Fatalf("isprintable surrogate = true, want false")
	}

	// With no hook the predicates degrade to Go's unicode tables, so ASCII still
	// classifies while the newer Garay capital is unknown to isalpha.
	CategoryHook = nil
	DigitHook = nil
	NumericHook = nil
	if !strPredicate("isalpha", "abc") {
		t.Fatalf("isalpha fallback abc = false, want true")
	}
	if strPredicate("isalpha", "\U00010D50") {
		t.Fatalf("isalpha fallback garay = true, want false")
	}
	if !strPredicate("isdigit", "9") {
		t.Fatalf("isdigit fallback 9 = false, want true")
	}
}

// TestStrIsidentifierProp covers str.isidentifier routing through the pinned
// XID_Start and XID_Continue hooks. The stubs add a newer-block letter Go's
// tables miss (the Garay small A) that starts and continues an identifier, and a
// combining mark that only continues one, so the predicate splits the two
// positions the way CPython does.
func TestStrIsidentifierProp(t *testing.T) {
	// ASCII letters, the underscore and the Garay small A start an identifier; the
	// ASCII digit and the combining acute continue one but do not start it.
	IDStartHook = func(r rune) bool {
		return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == 0x10D70
	}
	IDContinueHook = func(r rune) bool {
		return IDStartHook(r) || (r >= '0' && r <= '9') || r == 0x0301
	}
	defer func() {
		IDStartHook = nil
		IDContinueHook = nil
	}()

	runStrMCases(t, []strMCase{
		{"ident plain", "name", "isidentifier", nil, "True", ""},
		{"ident underscore", "_x1", "isidentifier", nil, "True", ""},
		{"ident leading digit", "1x", "isidentifier", nil, "False", ""},
		{"ident empty", "", "isidentifier", nil, "False", ""},
		{"ident space", "a b", "isidentifier", nil, "False", ""},
		// The Garay small letter starts and continues an identifier.
		{"ident garay", "\U00010D70x", "isidentifier", nil, "True", ""},
		{"ident garay start", "x\U00010D70", "isidentifier", nil, "True", ""},
		// The combining acute continues but does not start.
		{"ident mark cont", "á", "isidentifier", nil, "True", ""},
		{"ident mark start", "́a", "isidentifier", nil, "False", ""},
	})

	// With no hook the predicate degrades to Go's tables, so ASCII still works and
	// the newer Garay small letter is unknown to the start class.
	IDStartHook = nil
	IDContinueHook = nil
	if !strPredicate("isidentifier", "abc_1") {
		t.Fatalf("isidentifier fallback abc_1 = false, want true")
	}
	if strPredicate("isidentifier", "\U00010D70") {
		t.Fatalf("isidentifier fallback garay = true, want false")
	}
}
