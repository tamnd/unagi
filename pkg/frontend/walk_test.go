package frontend

import (
	"sort"
	"testing"
)

// callAttrs collects the attribute names of every attribute-call in src, so a
// test can assert WalkCalls reaches a call in a given syntactic position.
func callAttrs(t *testing.T, src string) []string {
	t.Helper()
	mod, err := Parse([]byte(src), "main.py")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var names []string
	WalkCalls(mod.Body, func(c *Call) {
		if at, ok := c.Fn.(*Attribute); ok {
			names = append(names, at.Name)
		}
	})
	sort.Strings(names)
	return names
}

// TestWalkCallsReachesEveryPosition checks WalkCalls descends into the statement
// and expression forms a codec call can hide in, so the seeding scan that rides
// on it cannot miss a str.encode wherever it appears.
func TestWalkCallsReachesEveryPosition(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"expression statement", `x.hit()`},
		{"assignment value", `y = x.hit()`},
		{"call argument", `print(x.hit())`},
		{"nested call", `f(g(x.hit()))`},
		{"return value", "def f():\n    return x.hit()\n"},
		{"if condition", "if x.hit():\n    pass\n"},
		{"for iterable", "for i in x.hit():\n    pass\n"},
		{"while condition", "while x.hit():\n    break\n"},
		{"with context", "with x.hit() as h:\n    pass\n"},
		{"try body", "try:\n    x.hit()\nexcept Exception:\n    pass\n"},
		{"list element", `z = [x.hit()]`},
		{"dict value", `z = {1: x.hit()}`},
		{"comprehension element", `z = [x.hit() for i in range(3)]`},
		{"comprehension iterable", `z = [i for i in x.hit()]`},
		{"f-string interpolation", `z = f"{x.hit()}"`},
		{"lambda body", `f = lambda: x.hit()`},
		{"default argument", "def f(a=x.hit()):\n    pass\n"},
		{"decorator", "@x.hit()\ndef f():\n    pass\n"},
		{"class base", "class C(x.hit()):\n    pass\n"},
		{"boolean operand", `z = a or x.hit()`},
		{"ternary arm", `z = x.hit() if a else b`},
		{"subscript index", `z = a[x.hit()]`},
		{"binary operand", `z = a + x.hit()`},
		{"walrus value", `z = (w := x.hit())`},
		{"nested method body", "class C:\n    def m(self):\n        return x.hit()\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := callAttrs(t, tc.src)
			found := false
			for _, n := range got {
				if n == "hit" {
					found = true
				}
			}
			if !found {
				t.Errorf("WalkCalls did not reach the call: got %v", got)
			}
		})
	}
}

// TestWalkCallsReportsEveryCall checks a call that nests another call reports
// both, so an outer str() constructor and an inner .encode() are each seen.
func TestWalkCallsReportsEveryCall(t *testing.T) {
	got := callAttrs(t, `outer(a.first(b.second()))`)
	want := []string{"first", "second"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("attribute calls = %v, want %v", got, want)
	}
}
