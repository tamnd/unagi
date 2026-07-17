package partition

import (
	"github.com/tamnd/unagi/pkg/frontend"
	"github.com/tamnd/unagi/pkg/ir"
)

// This file wires the call graph into the decision, the phase-three fixpoint doc
// 06 section 3.2 calls for. A function that calls another function can only be
// proven static once that callee is known static: until then the bridge refuses
// the call and the caller stays boxed. So the driver decides in rounds, each round
// feeding the previous round's proven static callees back in as a resolver the
// bridge lowers direct calls against. The proven set grows monotonically, since a
// callee once resolved never un-resolves, so the loop converges; the module's body
// count bounds it.
//
// A callee is offered to the resolver only when its static form actually ships:
// the unit decided static and its deopt plan is empty, the same guard-free gate
// build.staticForms emits under. A guarded static unit's twin is not built until
// the trampoline band, so a caller cannot yet call it directly and it is withheld.
// Only a top-level module function is a resolvable callee here; a method or nested
// function is named through an attribute or a closure the scalar subset does not
// lower, so the bare-name call site never resolves one.

// moduleCallees builds the resolver map for the next round from the current
// decision set. Every top-level function that decided static with an empty deopt
// plan becomes a callable static callee under its bare name, with the unboxed
// signature a caller needs to build a direct call. The signature is read from
// lowering the callee with the round's own resolver, so a callee that itself calls
// a static unit still reports the right shape. The result is a pure function of the
// decisions, the module, and the resolver, which keeps the fixpoint deterministic.
func moduleCallees(m *frontend.Module, decisions []Decision, resolve ir.CalleeResolver) map[string]ir.StaticCallee {
	staticFree := make(map[string]bool, len(decisions))
	for _, d := range decisions {
		if d.State.IsStatic() && len(d.Deopts) == 0 {
			staticFree[d.Unit.Name] = true
		}
	}
	tracked := ir.TrackedGlobals(m)
	out := map[string]ir.StaticCallee{}
	for _, s := range m.Body {
		fn, ok := s.(*frontend.FuncDef)
		if !ok {
			continue
		}
		if !staticFree[ModuleUnitName+"."+fn.Name] {
			continue
		}
		f, err := ir.LowerFuncFull(fn, resolve, ir.GlobalResolverFor(fn, tracked))
		if err != nil {
			// The unit decided static, so it lowers here too; a refusal would mean the
			// decision and the bridge disagree. Skip it rather than record a callee with
			// no signature, and the caller stays boxed, which is safe.
			continue
		}
		// The Go name is not load-bearing here: it feeds only the decision, which scores
		// off the operation census and never the callee name. build.staticForms builds
		// its own resolver with the emitted names when it lowers the call for real.
		out[fn.Name] = ir.SignatureOf(f, "static_"+fn.Name)
	}
	return out
}

// resolverFor turns a callee map into the resolver closure the bridge queries. An
// empty map returns a nil resolver, which resolves nothing, so the first round
// (before any callee is proven) runs exactly as the resolver-free driver did.
func resolverFor(callees map[string]ir.StaticCallee) ir.CalleeResolver {
	if len(callees) == 0 {
		return nil
	}
	return func(name string) (ir.StaticCallee, bool) {
		c, ok := callees[name]
		return c, ok
	}
}
