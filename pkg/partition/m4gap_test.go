package partition

import "testing"

// This file closes the last pkg/partition-level gap the M4 guards-and-deopt
// checklist (notes/Spec/2076/milestones/M4/09) had open: the verifier suite must
// be exhaustive over the malformations it defines, so a new violation code cannot
// be added without a test that provokes it. The six per-code tests live in
// deopt_test.go; this one asserts the roster is complete.

// TestVerifierCoversEveryViolation provokes each verifier violation code from a
// well-formed site mutated one way, and asserts the set of codes the suite can
// produce is exactly the declared roster. If a new ViolXxx code is added to
// deopt.go without a mutation here, the roster check fails, so the malformed-guard
// suite cannot silently fall behind the verifier.
func TestVerifierCoversEveryViolation(t *testing.T) {
	// Every violation code the verifier declares. Adding a code to deopt.go
	// without adding it here (and a mutation below) fails the completeness check.
	roster := []string{
		ViolGuardNotInterior,
		ViolResumeMidExpr,
		ViolEffectBeforeDeopt,
		ViolLiveVarUnmapped,
		ViolTransferNotLive,
		ViolPointerCopyBoxless,
	}

	// One mutation of a well-formed site per code, each of which the verifier must
	// flag with that code.
	provoke := map[string]func(s *DeoptSite){
		ViolGuardNotInterior:  func(s *DeoptSite) { s.Guard.Edge = EdgeRouteBoxed },
		ViolResumeMidExpr:     func(s *DeoptSite) { s.Resume.Kind = ResumeMidExpression },
		ViolEffectBeforeDeopt: func(s *DeoptSite) { s.EffectBefore = true },
		ViolLiveVarUnmapped:   func(s *DeoptSite) { s.LiveVars = append(s.LiveVars, "extra") },
		ViolTransferNotLive: func(s *DeoptSite) {
			s.Transfers = append(s.Transfers, TransferEntry{Slot: 9, Native: "ghost", Kind: MatRebox})
		},
		ViolPointerCopyBoxless: func(s *DeoptSite) {
			for i := range s.Transfers {
				if s.Transfers[i].Native == "rows" {
					s.Transfers[i].Escaped = false
				}
			}
		},
	}

	if len(provoke) != len(roster) {
		t.Fatalf("the roster lists %d codes but %d mutations are defined; keep them in lockstep", len(roster), len(provoke))
	}
	for _, code := range roster {
		mutate, ok := provoke[code]
		if !ok {
			t.Errorf("no malformed-site mutation provokes %q; the verifier suite is not exhaustive", code)
			continue
		}
		s := wellFormedSite()
		mutate(&s)
		if !hasViol(VerifyDeopt(s), code) {
			t.Errorf("mutation for %q did not produce that violation", code)
		}
	}
}
