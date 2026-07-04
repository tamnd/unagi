package objects

import (
	"strings"
	"testing"
)

func TestMatchSequenceKind(t *testing.T) {
	yes := []Object{L(NewInt(1)), T(NewInt(1)), NewRange(0, 3, 1)}
	for _, o := range yes {
		if !MatchSequence(o) {
			t.Errorf("MatchSequence(%s) = false, want true", Repr(o))
		}
	}
	no := []Object{NewStr("ab"), NewInt(1), D(t)}
	for _, o := range no {
		if MatchSequence(o) {
			t.Errorf("MatchSequence(%s) = true, want false", Repr(o))
		}
	}
}

func TestMatchMappingKind(t *testing.T) {
	if !MatchMapping(D(t)) {
		t.Error("MatchMapping(dict) = false, want true")
	}
	for _, o := range []Object{L(NewInt(1)), NewStr("x"), NewInt(1)} {
		if MatchMapping(o) {
			t.Errorf("MatchMapping(%s) = true, want false", Repr(o))
		}
	}
}

func TestMatchStar(t *testing.T) {
	seq := L(NewInt(0), NewInt(1), NewInt(2), NewInt(3), NewInt(4))
	got, err := MatchStar(seq, 1, 2)
	if err != nil {
		t.Fatalf("MatchStar: %v", err)
	}
	if r := Repr(got); r != "[1, 2]" {
		t.Errorf("MatchStar middle = %s, want [1, 2]", r)
	}
	// A star can bind zero elements when before+after spans the whole subject.
	empty, err := MatchStar(seq, 2, 3)
	if err != nil {
		t.Fatalf("MatchStar empty: %v", err)
	}
	if r := Repr(empty); r != "[]" {
		t.Errorf("MatchStar empty = %s, want []", r)
	}
}

func TestMatchKeysPresentAndMissing(t *testing.T) {
	d := D(t, NewStr("a"), NewInt(1), NewStr("b"), NewInt(2))
	vals, ok, err := MatchKeys(d, []Object{NewStr("b"), NewStr("a")})
	if err != nil || !ok {
		t.Fatalf("MatchKeys present: ok=%v err=%v", ok, err)
	}
	if Repr(vals[0]) != "2" || Repr(vals[1]) != "1" {
		t.Errorf("MatchKeys values = %s %s, want 2 1", Repr(vals[0]), Repr(vals[1]))
	}
	_, ok, err = MatchKeys(d, []Object{NewStr("a"), NewStr("z")})
	if err != nil {
		t.Fatalf("MatchKeys missing err: %v", err)
	}
	if ok {
		t.Error("MatchKeys with a missing key = ok, want not ok")
	}
	// A non-mapping subject never matches.
	if _, ok, _ := MatchKeys(L(NewInt(1)), []Object{NewStr("a")}); ok {
		t.Error("MatchKeys on a list = ok, want not ok")
	}
}

func TestMatchKeysDuplicate(t *testing.T) {
	d := D(t, NewStr("a"), NewInt(1))
	_, _, err := MatchKeys(d, []Object{NewStr("a"), NewStr("a")})
	if err == nil {
		t.Fatal("MatchKeys with duplicate key: want error")
	}
	want := "mapping pattern checks duplicate key ('a')"
	if got := err.Error(); !strings.Contains(got, want) {
		t.Errorf("MatchKeys duplicate error = %q, want substring %q", got, want)
	}
}

func TestMatchRestOrder(t *testing.T) {
	d := D(t, NewStr("a"), NewInt(1), NewStr("b"), NewInt(2), NewStr("c"), NewInt(3))
	rest, err := MatchRest(d, []Object{NewStr("b")})
	if err != nil {
		t.Fatalf("MatchRest: %v", err)
	}
	if r := Repr(rest); r != "{'a': 1, 'c': 3}" {
		t.Errorf("MatchRest = %s, want {'a': 1, 'c': 3}", r)
	}
	// Dropping every key leaves an empty dict, not the original.
	all, err := MatchRest(d, []Object{NewStr("a"), NewStr("b"), NewStr("c")})
	if err != nil {
		t.Fatalf("MatchRest all: %v", err)
	}
	if r := Repr(all); r != "{}" {
		t.Errorf("MatchRest all = %s, want {}", r)
	}
}
