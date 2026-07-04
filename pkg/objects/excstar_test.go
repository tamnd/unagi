package objects

import "testing"

// leaf builds a plain exception for the split trees below.
func leaf(kind, msg string) *Exception {
	return &Exception{Kind: kind, Args: []Object{NewStr(msg)}}
}

// group builds an ExceptionGroup around the given sub-exceptions.
func group(msg string, subs ...*Exception) *Exception {
	objs := make([]Object, len(subs))
	for i, s := range subs {
		objs[i] = s
	}
	return &Exception{Kind: "ExceptionGroup", Args: []Object{NewStr(msg), NewTuple(objs)}, Group: subs}
}

// kinds lists the class names of a group's direct children.
func kinds(e *Exception) []string {
	if e == nil {
		return nil
	}
	out := make([]string, len(e.Group))
	for i, s := range e.Group {
		out[i] = s.Kind
	}
	return out
}

func TestSplitStarPreservesStructure(t *testing.T) {
	// The tree and every partition below were probed against python3.14:
	// split keeps the group message on both halves and recurses into
	// nested groups, dropping the branches that went the other way.
	g := group("top",
		leaf("ValueError", "v1"),
		leaf("TypeError", "t1"),
		group("nested", leaf("ValueError", "v2"), leaf("KeyError", "k1")),
	)
	matched, rest := SplitStar(g, []string{"ValueError"})
	if matched == nil || matched.Kind != "ExceptionGroup" || Str(matched.Args[0]) != "top" {
		t.Fatalf("matched top = %v", matched)
	}
	if got := kinds(matched); len(got) != 2 || got[0] != "ValueError" || got[1] != "ExceptionGroup" {
		t.Fatalf("matched children = %v", got)
	}
	if k := kinds(matched.Group[1]); len(k) != 1 || k[0] != "ValueError" {
		t.Fatalf("matched nested = %v", k)
	}
	if rest == nil || Str(rest.Args[0]) != "top" {
		t.Fatalf("rest top = %v", rest)
	}
	if got := kinds(rest); len(got) != 2 || got[0] != "TypeError" || got[1] != "ExceptionGroup" {
		t.Fatalf("rest children = %v", got)
	}
	if k := kinds(rest.Group[1]); len(k) != 1 || k[0] != "KeyError" {
		t.Fatalf("rest nested = %v", k)
	}
}

func TestSplitStarNakedWrap(t *testing.T) {
	ve := leaf("ValueError", "solo")
	matched, rest := SplitStar(ve, []string{"ValueError"})
	if rest != nil {
		t.Fatalf("rest = %v, want nil", rest)
	}
	if matched == nil || matched.Kind != "ExceptionGroup" || Str(matched.Args[0]) != "" {
		t.Fatalf("matched = %v, want empty-message group", matched)
	}
	if len(matched.Group) != 1 || matched.Group[0] != ve {
		t.Fatalf("matched group = %v", matched.Group)
	}

	matched, rest = SplitStar(ve, []string{"KeyError"})
	if matched != nil {
		t.Fatalf("matched = %v, want nil", matched)
	}
	if rest != ve {
		t.Fatalf("rest = %v, want the naked exception unchanged", rest)
	}
}

func TestSplitStarFullMatch(t *testing.T) {
	g := group("g", leaf("ValueError", "v"))
	matched, rest := SplitStar(g, []string{"ValueError"})
	if rest != nil {
		t.Fatalf("rest = %v, want nil when everything matched", rest)
	}
	if matched == nil || len(matched.Group) != 1 {
		t.Fatalf("matched = %v", matched)
	}
}

func TestCombineStar(t *testing.T) {
	rest := group("g", leaf("KeyError", "k"))
	raised := leaf("RuntimeError", "boom")

	// Remainder alone propagates as itself.
	if got := CombineStar(rest, nil); got != rest {
		t.Fatalf("remainder-only = %v, want the remainder", got)
	}
	// Nothing left propagates as nil.
	if got := CombineStar(nil, nil); got != nil {
		t.Fatalf("empty = %v, want nil", got)
	}
	// A single raised exception with no remainder propagates bare.
	if got := CombineStar(nil, []*Exception{raised}); got != raised {
		t.Fatalf("single raised = %v, want the raised exception", got)
	}
	// Raised plus remainder wrap into an empty-message group, raised first.
	got := CombineStar(rest, []*Exception{raised})
	if got == nil || got.Kind != "ExceptionGroup" || Str(got.Args[0]) != "" {
		t.Fatalf("combined = %v, want empty-message group", got)
	}
	if len(got.Group) != 2 || got.Group[0] != raised || got.Group[1] != rest {
		t.Fatalf("combined children = %v", got.Group)
	}
}
