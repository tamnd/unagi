package types

import "testing"

// The worked join and meet evaluations from spec 2076 doc 04 section 3.8 are
// the contract for the algebra, so they are pinned literally here.

func TestJoinWorkedExamples(t *testing.T) {
	in := NewInterner()

	circle := in.Class("Circle", []string{"Circle", "Shape", "object"})
	square := in.Class("Square", []string{"Square", "Shape", "object"})
	shape := in.Class("Shape", []string{"Shape", "object"})

	cases := []struct {
		name string
		a, b *Type
		want string
	}{
		{"int join bool is int", in.Int(), in.Bool(), "int"},
		{"int join float stays a union", in.Int(), in.Float(), "int | float"},
		{"str join none is optional", in.Str(), in.None(), "None | str"},
		{"list elements join", in.List(in.Int()), in.List(in.Str()), "list[int | str]"},
		{"tuple arity mismatch goes variadic",
			in.Tuple(in.Int(), in.Str()), in.Tuple(in.Int(), in.Str(), in.Str()),
			"tuple[int | str, ...]"},
		{"fifth unrelated builtin widens to dyn",
			in.Union(in.Int(), in.Str(), in.Bytes(), in.Float()), in.Complex(),
			"Dyn"},
		{"classes join to common ancestor", circle, square, "Shape"},
		{"class join non-class widens to dyn", circle, in.Dict(in.Str(), in.Int()), "Dyn"},
		{"join with dyn absorbs", in.Int(), in.Dyn(), "Dyn"},
		{"join with never is identity", in.Never(), in.Str(), "str"},
		{"derived joins into base", circle, shape, "Shape"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := in.Join(c.a, c.b).String(); got != c.want {
				t.Fatalf("Join = %s, want %s", got, c.want)
			}
			// Join is commutative.
			if got := in.Join(c.b, c.a).String(); got != c.want {
				t.Fatalf("Join reversed = %s, want %s", got, c.want)
			}
		})
	}
}

func TestMeetWorkedExamples(t *testing.T) {
	in := NewInterner()

	cases := []struct {
		name string
		a, b *Type
		want string
	}{
		{"meet dyn with str is a guard", in.Dyn(), in.Str(), "str"},
		{"meet union by isinstance", in.Union(in.Int(), in.Str()), in.Str(), "str"},
		{"meet the is-not-none test", in.Optional(in.Int()), in.Int(), "int"},
		{"meet disjoint is never", in.Str(), in.Bytes(), "Never"},
		{"meet int with bool narrows to bool", in.Int(), in.Bool(), "bool"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := in.Meet(c.a, c.b).String(); got != c.want {
				t.Fatalf("Meet = %s, want %s", got, c.want)
			}
			if got := in.Meet(c.b, c.a).String(); got != c.want {
				t.Fatalf("Meet reversed = %s, want %s", got, c.want)
			}
		})
	}
}

func TestUnionBudgetAndCanonicalOrder(t *testing.T) {
	in := NewInterner()

	// Four unrelated builtins fit the budget and sort canonically.
	u := in.Union(in.Str(), in.Int(), in.Bytes(), in.Float())
	if u.Kind() != KindUnion || len(u.Elems()) != 4 {
		t.Fatalf("want a 4-member union, got %s", u)
	}
	// Same members in a different order intern to the same pointer.
	u2 := in.Union(in.Float(), in.Bytes(), in.Int(), in.Str())
	if u != u2 {
		t.Fatalf("union order changed identity: %s vs %s", u, u2)
	}

	// A fifth unrelated builtin blows the budget and widens to Dyn.
	if got := in.Union(u, in.Complex()).String(); got != "Dyn" {
		t.Fatalf("over-budget union = %s, want Dyn", got)
	}
}

func TestUnionCollapsesSubtypes(t *testing.T) {
	in := NewInterner()
	// bool is absorbed by int, so the union degenerates to int alone.
	if got := in.Union(in.Int(), in.Bool()).String(); got != "int" {
		t.Fatalf("int|bool = %s, want int", got)
	}
	// A class union widens to the common ancestor when it exceeds the budget.
	names := [][]string{
		{"A", "Base", "object"},
		{"B", "Base", "object"},
		{"C", "Base", "object"},
		{"D", "Base", "object"},
		{"E", "Base", "object"},
	}
	members := make([]*Type, len(names))
	for i, mro := range names {
		members[i] = in.Class(mro[0], mro)
	}
	if got := in.Union(members...).String(); got != "Base" {
		t.Fatalf("five sibling classes = %s, want Base", got)
	}
}

func TestOptionalIsUnionWithNone(t *testing.T) {
	in := NewInterner()
	opt := in.Optional(in.Str())
	if opt.Kind() != KindUnion {
		t.Fatalf("Optional should be a union, got %s", opt)
	}
	// Meeting away None recovers the base type.
	if got := in.Meet(opt, in.Str()).String(); got != "str" {
		t.Fatalf("narrowing optional = %s, want str", got)
	}
	// The is-None edge keeps only None.
	if got := in.Meet(opt, in.None()).String(); got != "None" {
		t.Fatalf("is-none edge = %s, want None", got)
	}
}
