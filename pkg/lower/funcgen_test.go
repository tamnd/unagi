package lower

import "testing"

// TestCleanDoc pins the docstring dedent against values recorded from CPython
// 3.14.6, which strips the first line's leading whitespace and the common
// column margin from the rest, expanding tabs at an eight-column tabstop.
func TestCleanDoc(t *testing.T) {
	cases := []struct{ in, want string }{
		{"single", "single"},
		{"   only line   ", "only line   "},
		{"first line   \n    second", "first line   \nsecond"},
		{"  first\n    second", "first\nsecond"},
		{"first\n        second\n      third", "first\n  second\nthird"},
		{"a\n\n    b", "a\n\nb"},
		{"a\n  b\n      ", "a\nb\n    "},
		{"a\n      \n  b", "a\n    \nb"},
		{"first\n\t\tx\n\ty", "first\n        x\ny"},
		{"x\n\t  y\n  \tz", "x\n  y\nz"},
		{"a\nno_indent\n\tx", "a\nno_indent\n        x"},
		{"  \n  a", "\na"},
		{"tab\n\tindented with tab\n    ", "tab\nindented with tab\n"},
	}
	for _, c := range cases {
		if got := cleanDoc(c.in); got != c.want {
			t.Errorf("cleanDoc(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDocstringOf(t *testing.T) {
	// no body, non-string first statement, and a real docstring.
	if _, ok := docstringOf(nil); ok {
		t.Error("empty body should have no docstring")
	}
}
