package partition

import (
	"maps"

	"github.com/tamnd/unagi/pkg/frontend"
	"github.com/tamnd/unagi/pkg/ir"
)

// This file adds the greatest-fixpoint seed a mutually recursive static cycle
// needs. The round-based resolver in resolve.go grows the proven-static set from
// an empty seed, one callee at a time, so a function is proven static only once
// its callees already are. That least fixpoint can never bootstrap a cycle: two
// functions that call each other are each waiting on the other, so neither is
// ever offered to the resolver and both stay boxed (doc 07, the mutually
// recursive SCC case).
//
// The cure is to decide a cycle from the top instead of the bottom. After the
// least fixpoint settles, every function still boxed but describable from its
// annotations becomes a candidate. The pass seeds the resolver with all
// candidates at once, so each one's calls into the others resolve, then re-runs
// the decision and drops any candidate that still did not prove static and
// guard-free. Dropping shrinks the seed, which can only unresolve more calls, so
// the set decreases monotonically to the largest self-consistent set: a cycle
// survives exactly when, assuming its members static, every member proves
// static. This is the greatest fixpoint, and it decides the whole SCC together.
//
// The seed is sound because a candidate's assumed signature is its annotation,
// and a member that survives proved static through the bridge, which rejects a
// body whose inferred return disagrees with its annotation. So a surviving
// member really has the shape its callers assumed, and a member that does not
// survive is removed before any caller keeps a direct call into it.

// promoteCycles runs the greatest-fixpoint seed over the decisions the least
// fixpoint produced. It leaves a module with no recursive static candidates
// untouched, so the common acyclic program pays only one extra map walk. The
// resolve argument is the converged resolver from the least fixpoint, the source
// of the already-proven guard-free callees the seed is layered on top of.
func promoteCycles(module string, m *frontend.Module, decisions []Decision, resolve ir.CalleeResolver, mode Mode) []Decision {
	base := moduleCallees(m, decisions, resolve)
	seeds := cycleSeeds(m, byUnitName(decisions))
	if len(seeds) == 0 {
		return decisions
	}
	for {
		decisions = driveOnce(module, m, resolverFor(mergeCallees(base, seeds)), mode)
		byName := byUnitName(decisions)
		removed := false
		for bare := range seeds {
			d := byName[ModuleUnitName+"."+bare]
			if !d.State.IsStatic() || len(d.Deopts) != 0 {
				delete(seeds, bare)
				removed = true
			}
		}
		if !removed {
			return decisions
		}
		if len(seeds) == 0 {
			// Every candidate fell out, so the seeded resolver collapsed back to the
			// base one. Re-run once against the base alone so the returned decisions
			// match a plain least-fixpoint build rather than a stale seeded pass.
			return driveOnce(module, m, resolverFor(base), mode)
		}
	}
}

// cycleSeeds collects the seed signatures for every top-level function that the
// least fixpoint left boxed but whose annotations describe a scalar signature. A
// function already proven static and guard-free is not a candidate: it is
// already a base callee, so seeding it would be redundant. A function the
// annotations cannot describe (an unannotated return, a non-scalar parameter) is
// skipped, since a caller could not build a direct call into it anyway, so it
// could never be the static end of a cycle at M4.
func cycleSeeds(m *frontend.Module, byName map[string]Decision) map[string]ir.StaticCallee {
	seeds := map[string]ir.StaticCallee{}
	for _, s := range m.Body {
		fn, ok := s.(*frontend.FuncDef)
		if !ok {
			continue
		}
		d, ok := byName[ModuleUnitName+"."+fn.Name]
		if !ok {
			continue
		}
		if d.State.IsStatic() && len(d.Deopts) == 0 {
			continue
		}
		sig, ok := ir.SignatureFromDef(fn, "static_"+fn.Name)
		if !ok {
			continue
		}
		seeds[fn.Name] = sig
	}
	return seeds
}

// mergeCallees layers the seed signatures over the base callees into a fresh map,
// leaving both inputs untouched so the fixpoint loop can shrink the seed without
// disturbing the base. A seed and a base entry never collide, since a candidate
// is boxed and a base callee is static, but the seed is written last regardless
// so the intent, a seeded signature taking precedence, is explicit.
func mergeCallees(base, seeds map[string]ir.StaticCallee) map[string]ir.StaticCallee {
	out := make(map[string]ir.StaticCallee, len(base)+len(seeds))
	maps.Copy(out, base)
	maps.Copy(out, seeds)
	return out
}

// byUnitName indexes a decision set by its qualified unit name for the fixpoint's
// per-round lookups.
func byUnitName(decisions []Decision) map[string]Decision {
	out := make(map[string]Decision, len(decisions))
	for _, d := range decisions {
		out[d.Unit.Name] = d
	}
	return out
}
