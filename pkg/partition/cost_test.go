package partition

import "testing"

func TestScoreArithmetic(t *testing.T) {
	// A pure numeric loop: many unboxed ops, one hoisted binding guard, no boxed
	// ops and no thunks. The static score should dwarf 60 percent of the boxed
	// twin, so it emits static.
	p := Profile{UnboxedOps: 40, EntryGuards: 1}
	s := DefaultWeights.Score(p)
	if s.Static != 40*1+1*2 {
		t.Fatalf("static score = %d, want 42", s.Static)
	}
	if s.Boxed != 40*8 {
		t.Fatalf("boxed score = %d, want 320", s.Boxed)
	}
	if !s.EmitStatic() {
		t.Fatalf("a numeric loop should emit static (%d vs %d)", s.Static, s.Boxed)
	}
}

func TestScoreThunkHeavyLoses(t *testing.T) {
	// A unit whose every op is a static-to-boxed crossing wins nothing; the cost
	// model votes to demote it.
	p := Profile{UnboxedOps: 4, ThunkCross: 4}
	s := DefaultWeights.Score(p)
	if s.EmitStatic() {
		t.Fatalf("a thunk-dominated unit should not emit static (%d vs %d)", s.Static, s.Boxed)
	}
}

func TestExcursionBudget(t *testing.T) {
	// Exactly 25 percent boxed fits; over 25 percent does not.
	ok := Profile{UnboxedOps: 30, ExcursionOps: 10} // 10 of 40 = 25 percent
	if !ok.ExcursionBudgetOK() {
		t.Fatalf("25 percent excursions should fit the budget")
	}
	over := Profile{UnboxedOps: 30, ExcursionOps: 11} // 11 of 41 > 25 percent
	if over.ExcursionBudgetOK() {
		t.Fatalf("over 25 percent excursions should exceed the budget")
	}
	// An empty unit trivially fits.
	if !(Profile{}).ExcursionBudgetOK() {
		t.Fatalf("empty profile should fit the budget")
	}
}

func TestEmitStaticBoundary(t *testing.T) {
	// Static exactly at 60 percent of boxed does not emit; strictly below does.
	// boxed = 100, so the threshold is static < 60.
	at := Score{Static: 60, Boxed: 100}
	if at.EmitStatic() {
		t.Fatalf("static at exactly 60 percent should not emit")
	}
	below := Score{Static: 59, Boxed: 100}
	if !below.EmitStatic() {
		t.Fatalf("static below 60 percent should emit")
	}
}
