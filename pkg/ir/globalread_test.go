package ir

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/emit"
)

// intGlobals is a GlobalResolver that reports the named globals as int-typed
// shadows and rejects everything else, standing in for the build's real resolver.
func intGlobals(names ...string) GlobalResolver {
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	return func(name string) (emit.Repr, bool) {
		if set[name] {
			return emit.Repr{Go: "int64", Scalar: emit.SInt}, true
		}
		return emit.Repr{}, false
	}
}

// TestLowerReadsTrackedGlobalThroughShadow checks that a free name the global
// resolver accepts lowers to a read of that global's typed shadow and that the
// function carries the matching entry world-age guard. The read against the
// shadow, not the Python name, is what the lower tier's package variable holds,
// and the guard is what keeps the read off a stale binding.
func TestLowerReadsTrackedGlobalThroughShadow(t *testing.T) {
	fn := parseFunc(t, "def boost(x: int) -> int:\n    return x * FACTOR\n")
	f, err := LowerFuncFull(fn, nil, intGlobals("FACTOR"), nil)
	if err != nil {
		t.Fatalf("LowerFuncFull: %v", err)
	}
	if len(f.BindingGuards) != 1 || f.BindingGuards[0].VerVar != "bver_FACTOR" || f.BindingGuards[0].Version != 1 {
		t.Fatalf("BindingGuards = %+v, want one {bver_FACTOR, 1}", f.BindingGuards)
	}
	f.DeoptHandler = "static_boost_deopt"
	got, err := emit.EmitFunc(f)
	if err != nil {
		t.Fatalf("EmitFunc: %v", err)
	}
	for _, want := range []string{
		"if bver_FACTOR != 1 {",
		"return static_boost_deopt(d0)",
		"rt.MulInt64(x, bshadow_FACTOR)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("emitted global read is missing %q:\n%s", want, got)
		}
	}
}

// TestTrackedGlobalReadIsOneEntryGuardSite checks that a function reading tracked
// globals opens exactly one deopt site, at entry, whose live set is the entry
// parameters. The single site is what keeps the unit off the guard-free
// direct-call path and gives its boxed twin the parameters to re-run from the top,
// and reading two globals still opens just the one entry site they share. The body
// is a comparison rather than arithmetic so no overflow guard opens a second site
// on top of the entry one, isolating the binding guard under test.
func TestTrackedGlobalReadIsOneEntryGuardSite(t *testing.T) {
	fn := parseFunc(t, "def combine(x: int) -> bool:\n    return A < B\n")
	f, err := LowerFuncFull(fn, nil, intGlobals("A", "B"), nil)
	if err != nil {
		t.Fatalf("LowerFuncFull: %v", err)
	}
	if len(f.BindingGuards) != 2 {
		t.Fatalf("BindingGuards = %+v, want two", f.BindingGuards)
	}
	sites := GuardSitesOf(f)
	if len(sites) != 1 {
		t.Fatalf("GuardSitesOf = %d sites, want 1 entry site", len(sites))
	}
	if len(sites[0].Live) != 1 || sites[0].Live[0].Name != "x" {
		t.Fatalf("entry site live set = %+v, want just the parameter x", sites[0].Live)
	}
}

// TestUntrackedFreeNameStillRefuses checks that a nil global resolver leaves the
// old behavior intact: a free name has no static form and keeps the unit boxed,
// so a function reading an unknown global does not silently lower against a
// shadow that was never declared.
func TestUntrackedFreeNameStillRefuses(t *testing.T) {
	fn := parseFunc(t, "def read() -> int:\n    return MISSING\n")
	if _, err := LowerFuncFull(fn, nil, nil, nil); err == nil {
		t.Fatal("LowerFuncFull lowered a free name with no global resolver, want refusal")
	}
	if _, err := LowerFuncFull(fn, nil, intGlobals("OTHER"), nil); err == nil {
		t.Fatal("LowerFuncFull lowered a name the resolver rejects, want refusal")
	}
}
