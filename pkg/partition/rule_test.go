package partition

import "testing"

func TestCatalogLookup(t *testing.T) {
	r, ok := LookupRule(RuleEvalDynamicSource)
	if !ok {
		t.Fatalf("eval rule should be registered")
	}
	if !r.Hard || r.Guardable {
		t.Fatalf("eval-dynamic-source should be hard and not guardable, got %+v", r)
	}
	if r.Scope != ScopeProgram {
		t.Fatalf("eval scope = %s, want program", r.Scope)
	}
}

func TestCatalogGuardableBinding(t *testing.T) {
	// A cross-module rebind is soft and guardable: a binding guard covers it.
	r := MustRule(RuleCrossModuleRebind)
	if r.Hard || !r.Guardable || r.Scope != ScopeBinding {
		t.Fatalf("cross-module-rebind should be soft, guardable, binding-scoped, got %+v", r)
	}
}

func TestCatalogUnknownPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("MustRule on an unknown id should panic")
		}
	}()
	MustRule("no-such-rule")
}

func TestCatalogAllRulesSorted(t *testing.T) {
	all := AllRules()
	if len(all) == 0 {
		t.Fatalf("catalog should not be empty")
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].ID >= all[i].ID {
			t.Fatalf("AllRules not sorted at %d: %q then %q", i, all[i-1].ID, all[i].ID)
		}
	}
	// Every rule has prose and a section, the report and coverage invariant both
	// depend on it.
	for _, r := range all {
		if r.Prose == "" || r.Section == "" {
			t.Fatalf("rule %q missing prose or section", r.ID)
		}
	}
}

func TestScopeString(t *testing.T) {
	cases := map[Scope]string{
		ScopeUnit: "unit", ScopeRegion: "region", ScopeClass: "class",
		ScopeBinding: "binding", ScopeModule: "module", ScopeProgram: "program",
	}
	for s, want := range cases {
		if s.String() != want {
			t.Fatalf("scope %d = %q, want %q", s, s.String(), want)
		}
	}
}
