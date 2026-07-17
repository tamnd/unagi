package build

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tamnd/unagi/pkg/emit"
	"github.com/tamnd/unagi/pkg/frontend"
	"github.com/tamnd/unagi/pkg/ir"
	"github.com/tamnd/unagi/pkg/lower"
	"github.com/tamnd/unagi/pkg/partition"
)

// This file lights the static tier into the build. The partitioner proves some
// functions static, but until now every function still shipped only in its boxed
// form; the static Go the emitter can produce went nowhere near a real module.
// Here a provable function's static form is emitted alongside the boxed module,
// so the two live in one binary. At M4 the static form is not yet called (the
// boxed tier still drives execution), so it is dead code for now; wiring the
// calls is the next band. What this slice buys is the first time ir.LowerFunc and
// emit.EmitFunc output passes through the Go toolchain on the real build path, so
// a static form that does not compile fails the build instead of passing a golden
// test in isolation.
//
// A guard-free static unit emits its static form directly. A unit that carries an
// overflow guard emits its static form plus a deopt edge into a boxed twin, which
// the trampoline band builds: the edge reboxes the entry parameters and re-runs
// the unit boxed from the top. That parameter replay is sound only when the
// decision's deopt plan is well formed, so a guarded unit earns a static form only
// when VerifyPlan clears its transfer table; an unsound plan keeps the unit
// boxed-only, where the answer is always correct.

// staticPlan is the shared static-tier layout for one module: the guard-free
// provable units, their fixed emitted Go names, and the resolver a caller lowers
// direct calls against. Both the static-form file and the entry-shim map read it,
// so a call site and the function it names always agree on the emitted name.
type staticPlan struct {
	funcs      []qualFunc
	staticFree map[string]bool
	deopt      map[string]bool
	names      map[string]string
	resolve    ir.CalleeResolver
	// tracked is the whole-module table of scalar globals a static form may read
	// through its shadow, keyed by name to scalar type. Every lowering pass in the
	// plan hands the bridge a per-function resolver derived from it, so a function
	// proven static during partitioning lowers the same tracked read here.
	tracked map[string]string
	// shapes is the whole-module table of fixed-shape classes the static tier gives
	// a Go struct, keyed by class name to its struct representation. The static file
	// declares one struct per entry so a form typed against the class has a real
	// type to name; the pass that types forms against them lands in a later slice.
	shapes map[string]emit.Repr
	// resume holds the mid-loop resume plan for each deopt-target unit whose
	// counting loop is provably safe to re-enter at the failing iteration. A unit
	// absent here keeps the from-top deopt edge, which is always correct.
	resume map[string]*resumeShape
}

// planStatic builds the static plan from the partitioner's decisions. A
// guard-free static unit is one it proved static and left with an empty deopt
// plan; a deopt-target static unit is one it proved static that carries an
// overflow guard, so its plan is non-empty. Both index by qualified name and take
// a fixed emitted Go name in source order, so the two emission passes name the
// same functions. A guard-free unit is a resolvable static-to-static callee; a
// deopt-target unit is not, since a static caller cannot thread its boxed deopt
// result, so it is reachable only from boxed code through its entry shim. It
// returns nil when no unit qualifies, so the build carries no static tier at all.
func planStatic(mod *frontend.Module, decisions []partition.Decision) *staticPlan {
	staticFree := make(map[string]bool, len(decisions))
	deoptTarget := make(map[string]bool, len(decisions))
	sites := make(map[string][]partition.DeoptSite, len(decisions))
	for _, d := range decisions {
		if !d.State.IsStatic() {
			continue
		}
		if len(d.Deopts) == 0 {
			staticFree[d.Unit.Name] = true
		} else {
			deoptTarget[d.Unit.Name] = true
			sites[d.Unit.Name] = d.Deopts
		}
	}
	if len(staticFree) == 0 && len(deoptTarget) == 0 {
		return nil
	}
	var funcs []qualFunc
	collectFuncs(partition.ModuleUnitName, mod.Body, &funcs)
	names := map[string]string{}
	seen := map[string]bool{}
	for _, qf := range funcs {
		if staticFree[qf.qual] || deoptTarget[qf.qual] {
			names[qf.qual] = staticName(qf.qual, seen)
		}
	}
	tracked := ir.TrackedGlobals(mod)
	shapes := ir.TrackedShapes(mod)
	plan := &staticPlan{
		funcs:      funcs,
		staticFree: staticFree,
		names:      names,
		tracked:    tracked,
		shapes:     shapes,
		resolve:    staticResolver(funcs, staticFree, names, tracked, shapes),
	}
	// A deopt-target unit only earns a static form once it also earns an entry
	// shim: the form's deopt edge tail-calls a hand-off the shim machinery emits,
	// and a boxed caller reaches the form only through that shim. So the emitted
	// set is exactly the top-level deopt-target units whose signature the shim can
	// cross; the rest stay boxed-only. Both emission passes read this set, so the
	// static form, its shim, and its hand-off are always emitted together.
	plan.deopt = map[string]bool{}
	plan.resume = map[string]*resumeShape{}
	for _, qf := range funcs {
		if !deoptTarget[qf.qual] {
			continue
		}
		if _, ok := topLevelName(qf.qual); !ok {
			continue
		}
		// The static form's deopt edge reboxes the entry parameters into the boxed
		// twin, which re-runs the unit from the top. That parameter replay reproduces
		// the boxed frame the tier would hold only when no observable effect precedes
		// any guard and every live local maps to a rebox transfer, which is exactly
		// what VerifyPlan checks over the site's transfer table. A plan that fails it
		// would materialize a frame the boxed tier never held and answer wrong on
		// deopt, so the unit demotes to boxed-only rather than ship an unsound form.
		if len(partition.VerifyPlan(sites[qf.qual])) != 0 {
			continue
		}
		if _, ok := plan.shimEntryFor(qf); ok {
			plan.deopt[qf.qual] = true
			// A deopt-target unit whose loop is the canonical single-accumulator
			// counting loop resumes mid-loop instead of replaying from the top. The
			// shape proof is structural on the def, so it rides alongside the shim
			// gate; a unit that does not fit keeps the from-top edge.
			if shape, ok := resumeShapeFor(qf.def); ok {
				plan.resume[qf.qual] = shape
			}
		}
	}
	return plan
}

// shimEntryFor lowers one function and translates its signature into the entry
// the shim consumes, reporting false when the function does not lower or steps
// outside the scalar subset the shim crosses. It is the shared gate the plan uses
// to decide which units earn a static form and shim.
func (plan *staticPlan) shimEntryFor(qf qualFunc) (lower.StaticEntry, bool) {
	f, err := ir.LowerFuncFull(qf.def, plan.resolve, ir.GlobalResolverFor(qf.def, plan.tracked), ir.ShapeResolverFor(plan.shapes))
	if err != nil {
		return lower.StaticEntry{}, false
	}
	sc := ir.SignatureOf(f, plan.names[qf.qual])
	return shimEntry(sc, plan.names[qf.qual])
}

// staticEntries builds the entry-shim map the boxed lowering routes through: a
// top-level guard-free static function keyed by its bare name, carrying its
// emitted static Go name and the scalar kinds the shim guards, unboxes, and
// reboxes. A function whose signature steps outside the scalar subset the shim
// handles is left out, so its boxed call stays boxed. It returns nil when no
// function qualifies, which leaves the boxed lowering exactly as it was.
func staticEntries(plan *staticPlan) map[string]lower.StaticEntry {
	if plan == nil {
		return nil
	}
	out := map[string]lower.StaticEntry{}
	for _, qf := range plan.funcs {
		if !plan.staticFree[qf.qual] && !plan.deopt[qf.qual] {
			continue
		}
		bare, ok := topLevelName(qf.qual)
		if !ok {
			continue
		}
		entry, ok := plan.shimEntryFor(qf)
		if !ok {
			continue
		}
		// A deopt-target unit's shim also unwraps the deopt sentinel its static
		// form returns, and the build emits the hand-off the sentinel comes from.
		entry.Deopt = plan.deopt[qf.qual]
		// A resume-eligible unit also carries the mid-loop resume plan: the boxed
		// twin that re-enters the loop and the hand-off that reboxes the counter,
		// the accumulator, and the entry parameters into it.
		if shape := plan.resume[qf.qual]; shape != nil {
			static := plan.names[qf.qual]
			entry.Resume = &lower.ResumePlan{
				Handler:  static + "_resume",
				TwinName: static + "_resume_twin",
				Twin:     shape.twin,
				Lead:     []lower.StaticScalar{lower.StaticInt, lower.StaticInt},
			}
		}
		out[bare] = entry
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// shimEntry translates a static callee's unboxed signature into the entry the
// shim consumes, reporting false when any parameter or the result is a
// representation the shim does not handle (a list, or a shape whose slot steps
// outside the scalar set). A scalar parameter the shim unboxes; a fixed-shape
// parameter the shim materializes from a boxed instance into a Go struct. The
// result must be a scalar the shim reboxes.
func shimEntry(sc ir.StaticCallee, static string) (lower.StaticEntry, bool) {
	params := make([]lower.StaticParam, len(sc.Params))
	for i, r := range sc.Params {
		p, ok := shimParam(r)
		if !ok {
			return lower.StaticEntry{}, false
		}
		params[i] = p
	}
	ret, ok := shimScalar(sc.Ret.Scalar)
	if !ok {
		return lower.StaticEntry{}, false
	}
	return lower.StaticEntry{Static: static, Params: params, Ret: ret}, true
}

// shimParam maps one parameter's representation onto the entry the shim consumes.
// A shaped representation becomes a shape the shim materializes, a scalar becomes
// a scalar it unboxes, and anything else (a list) reports false, which keeps the
// whole function boxed.
func shimParam(r emit.Repr) (lower.StaticParam, bool) {
	if r.Shape != nil {
		sh, ok := shimShape(r.Shape)
		if !ok {
			return lower.StaticParam{}, false
		}
		return lower.StaticParam{Shape: sh}, true
	}
	s, ok := shimScalar(r.Scalar)
	if !ok {
		return lower.StaticParam{}, false
	}
	return lower.ScalarParam(s), true
}

// shimShape translates a shape representation into the shim's shape descriptor:
// the class name the shape guard matches and the Go struct type the static form
// takes, plus each field's slot name and the scalar kind the shim unboxes it to.
// A field outside the scalar set leaves the whole shape off the static path,
// though the shape analysis admits only scalar slots, so that guard never trips
// in practice.
func shimShape(sh *emit.Shape) (*lower.StaticShape, bool) {
	fields := make([]lower.StaticShapeField, len(sh.Fields))
	for i, f := range sh.Fields {
		s, ok := shimScalar(f.Repr.Scalar)
		if !ok {
			return nil, false
		}
		fields[i] = lower.StaticShapeField{Name: f.Name, Scalar: s}
	}
	return &lower.StaticShape{Name: sh.Name, Fields: fields}, true
}

// shimScalar maps an emit scalar class onto the shim's scalar kind, reporting
// false for a non-scalar representation the shim cannot cross.
func shimScalar(s emit.Scalar) (lower.StaticScalar, bool) {
	switch s {
	case emit.SInt:
		return lower.StaticInt, true
	case emit.SFloat:
		return lower.StaticFloat, true
	case emit.SBool:
		return lower.StaticBool, true
	case emit.SStr:
		return lower.StaticStr, true
	}
	return lower.StaticNone, false
}

// staticForms renders the static-tier Go for every guard-free provable unit in
// the module, as one `package main` file to sit next to the boxed main.go. It
// returns nil when the plan is empty, so the build writes the file only when
// there is a static form to carry. The units are emitted in source order, which
// the plan's function walk follows, so the output is deterministic.
func staticForms(plan *staticPlan) ([]byte, error) {
	if plan == nil {
		return nil, nil
	}
	var b strings.Builder
	b.WriteString("// Code generated by unagi. DO NOT EDIT.\npackage main\n")
	var forms strings.Builder
	emitted := 0
	for _, qf := range plan.funcs {
		if !plan.staticFree[qf.qual] && !plan.deopt[qf.qual] {
			continue
		}
		f, err := ir.LowerFuncFull(qf.def, plan.resolve, ir.GlobalResolverFor(qf.def, plan.tracked), ir.ShapeResolverFor(plan.shapes))
		if err != nil {
			// A unit the partitioner proved static lowers here too; a lowering
			// failure means the decision and the bridge disagree, which is a bug
			// worth surfacing rather than silently dropping the form.
			return nil, fmt.Errorf("static unit %s decided static but did not lower: %w", qf.qual, err)
		}
		f.Name = plan.names[qf.qual]
		// A deopt-target form routes every overflow guard's failure edge to the
		// hand-off the shim machinery emits under this name, so a guard that fails
		// lands in a real function rather than the placeholder the goldens carry.
		if plan.deopt[qf.qual] {
			f.DeoptHandler = f.Name + "_deopt"
			// A resume-eligible unit routes its loop-body guard to the mid-loop
			// hand-off instead of the from-top one, so the failing activation
			// re-enters the boxed twin at the current iteration. The counter and
			// accumulator are named on the loop node the emitter walks.
			if shape := plan.resume[qf.qual]; shape != nil {
				setLoopResume(f.Body, f.Name+"_resume", shape.acc)
			}
		}
		src, err := emit.EmitFunc(f)
		if err != nil {
			return nil, fmt.Errorf("static unit %s: %w", qf.qual, err)
		}
		forms.WriteString("\n")
		forms.WriteString(src)
		forms.WriteString("\n")
		emitted++
	}
	if emitted == 0 {
		// Every static unit was a lambda or comprehension the function walk does
		// not surface, so there is nothing to write after all.
		return nil, nil
	}
	// The static file declares one Go struct per module shape class, the type a
	// form typed against that class names and an attribute read loads a field of.
	// The declarations are unused until a later slice types a form against them,
	// which is legal Go, so the file carries every module shape whether or not a
	// form references it yet. They emit in class-name order so the output is
	// deterministic.
	decls, err := shapeDecls(plan.shapes)
	if err != nil {
		return nil, err
	}
	// The static tier reaches its overflow helpers through rt; import it only when
	// a form actually names one, so a module of purely total forms stays
	// import-clean.
	if strings.Contains(forms.String(), runtimeQualifier+".") {
		fmt.Fprintf(&b, "\nimport %s %q\n", runtimeQualifier, runtimeImportPath)
	}
	b.WriteString(decls)
	b.WriteString(forms.String())
	return []byte(b.String()), nil
}

// shapeDecls renders the module's fixed-shape classes as Go struct type
// declarations, in class-name order so the emitted file is deterministic. An
// empty table renders nothing, so a module with no shape class carries no struct.
func shapeDecls(shapes map[string]emit.Repr) (string, error) {
	if len(shapes) == 0 {
		return "", nil
	}
	names := make([]string, 0, len(shapes))
	for name := range shapes {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		r := shapes[name]
		if r.Shape == nil {
			continue
		}
		src, err := emit.EmitShape(*r.Shape)
		if err != nil {
			return "", fmt.Errorf("shape %s: %w", name, err)
		}
		b.WriteString("\n")
		b.WriteString(src)
		b.WriteString("\n")
	}
	return b.String(), nil
}

// readGlobals is the subset of the module's tracked scalar globals that an
// emitted static form actually reads through its shadow, keyed by name to scalar
// type. It walks the same emitted-form set staticForms renders, lowers each, and
// unions the world-age guard the bridge attached, mapping each guard's version
// variable (bver_<name>) back to the global's name. The build hands this subset,
// not the whole tracked table, to the boxed lowering, so only a global a static
// form reads gets its shadow declared and its boxed stores instrumented; a
// tracked global no static form reads carries no shadow and no Rebind bump. It
// returns nil when no emitted form reads a tracked global.
func readGlobals(plan *staticPlan) map[string]string {
	if plan == nil || len(plan.tracked) == 0 {
		return nil
	}
	out := map[string]string{}
	for _, qf := range plan.funcs {
		if !plan.staticFree[qf.qual] && !plan.deopt[qf.qual] {
			continue
		}
		f, err := ir.LowerFuncFull(qf.def, plan.resolve, ir.GlobalResolverFor(qf.def, plan.tracked), ir.ShapeResolverFor(plan.shapes))
		if err != nil {
			continue
		}
		for _, g := range f.BindingGuards {
			name := strings.TrimPrefix(g.VerVar, "bver_")
			if scalar, ok := plan.tracked[name]; ok {
				out[name] = scalar
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// runtimeQualifier is the import alias the emitted static tier reaches its
// runtime helpers through, matching the alias pkg/emit prints. runtimeImportPath
// is the package that alias binds to.
const (
	runtimeQualifier  = "rt"
	runtimeImportPath = "github.com/tamnd/unagi/pkg/runtime"
)

// staticResolver builds the callee resolver the emit loop lowers direct calls
// against. A call site names a callee by a bare name, which reaches a top-level
// module function, so only a guard-free static top-level function is a resolvable
// callee, mapped from its bare name to its emitted Go name and unboxed signature.
// The signatures settle in a fixpoint: a callee that itself calls a static unit
// only lowers once that unit is in the resolver, so the loop grows the map until
// it stops, at most one entry per pass. This mirrors the partitioner's own call
// graph fixpoint, so the set of resolvable callees here matches the set the
// decision proved static, which is why every guard-free static caller lowers.
func staticResolver(funcs []qualFunc, staticFree map[string]bool, names map[string]string, tracked map[string]string, shapes map[string]emit.Repr) ir.CalleeResolver {
	callees := map[string]ir.StaticCallee{}
	resolve := func(name string) (ir.StaticCallee, bool) {
		c, ok := callees[name]
		return c, ok
	}
	// Seed each guard-free static callee from its annotations before the refinement
	// loop. The loop grows the map by lowering, one callee per pass, which never
	// bootstraps a mutually recursive cycle: each member's lowering waits on the
	// other's signature, so neither ever lowers and both drop out of the static
	// file, which fails the "decided static but did not lower" check. The seed
	// breaks the cycle, and for a scalar-signatured static unit the annotation
	// signature is exactly what SignatureOf reads off the lowered body, since the
	// bridge takes the same reprs from the same annotations, so the seed never
	// disagrees with the refined signature the loop would compute. A unit with no
	// scalar annotation to seed from is acyclic and lowers on its own in the loop.
	for _, qf := range funcs {
		if !staticFree[qf.qual] {
			continue
		}
		bare, ok := topLevelName(qf.qual)
		if !ok {
			continue
		}
		if sig, ok := ir.SignatureFromDef(qf.def, names[qf.qual]); ok {
			callees[bare] = sig
		}
	}
	for {
		grew := false
		for _, qf := range funcs {
			if !staticFree[qf.qual] {
				continue
			}
			bare, ok := topLevelName(qf.qual)
			if !ok {
				continue
			}
			if _, done := callees[bare]; done {
				continue
			}
			f, err := ir.LowerFuncFull(qf.def, resolve, ir.GlobalResolverFor(qf.def, tracked), ir.ShapeResolverFor(shapes))
			if err != nil {
				// The callee calls a static unit not yet in the resolver; a later pass
				// resolves it. A callee that never lowers is not top-level guard-free
				// static, which the decision already ruled out, so the loop still settles.
				continue
			}
			callees[bare] = ir.SignatureOf(f, names[qf.qual])
			grew = true
		}
		if !grew {
			return resolve
		}
	}
}

// topLevelName strips the module marker from a qualified unit name and reports the
// bare name plus whether the unit is a top-level module function. A nested
// function or method keeps a dotted tail and is not reachable by a bare-name call,
// so it is not a resolvable callee.
func topLevelName(qual string) (string, bool) {
	bare, ok := strings.CutPrefix(qual, partition.ModuleUnitName+".")
	if !ok || strings.Contains(bare, ".") {
		return "", false
	}
	return bare, true
}

// qualFunc pairs a function definition with the qualified unit name the
// partitioner keys its decision under, so a candidate's static decision can be
// found by name.
type qualFunc struct {
	qual string
	def  *frontend.FuncDef
}

// collectFuncs walks a statement list gathering every function definition with
// its qualified unit name, descending into nested functions, classes, and the
// control-flow blocks a definition can sit inside. The qualified names it builds
// match the partition driver's exactly (prefix + "." + name), so a decision looks
// up by the same key the driver filed it under. A lambda or comprehension is not
// a FuncDef and does not lower through the scalar bridge, so the walk does not
// surface one.
func collectFuncs(prefix string, body []frontend.Stmt, out *[]qualFunc) {
	for _, s := range body {
		switch s := s.(type) {
		case *frontend.FuncDef:
			child := prefix + "." + s.Name
			*out = append(*out, qualFunc{qual: child, def: s})
			collectFuncs(child, s.Body, out)
		case *frontend.ClassDef:
			child := prefix + "." + s.Name
			collectFuncs(child, s.Body, out)
		case *frontend.If:
			collectFuncs(prefix, s.Body, out)
			collectFuncs(prefix, s.Else, out)
		case *frontend.While:
			collectFuncs(prefix, s.Body, out)
			collectFuncs(prefix, s.Else, out)
		case *frontend.For:
			collectFuncs(prefix, s.Body, out)
			collectFuncs(prefix, s.Else, out)
		case *frontend.With:
			collectFuncs(prefix, s.Body, out)
		case *frontend.Try:
			collectFuncs(prefix, s.Body, out)
			for _, h := range s.Handlers {
				collectFuncs(prefix, h.Body, out)
			}
			collectFuncs(prefix, s.OrElse, out)
			collectFuncs(prefix, s.Final, out)
		case *frontend.Match:
			for _, c := range s.Cases {
				collectFuncs(prefix, c.Body, out)
			}
		}
	}
}

// staticName mangles a qualified unit name into a unique Go identifier for the
// emitted static function. It strips the module marker, folds every character a
// Go identifier cannot hold to an underscore, and prefixes "static" so the name
// never collides with a boxed identifier. Two qualified names can mangle to the
// same identifier (a nested `a.b` and a flat `a_b`), so a name already taken gets
// a numeric suffix; the walk is source-ordered, so the suffixing is deterministic.
func staticName(qual string, seen map[string]bool) string {
	trimmed := strings.TrimPrefix(qual, partition.ModuleUnitName)
	var b strings.Builder
	b.WriteString("static")
	for _, r := range trimmed {
		if r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	base := b.String()
	name := base
	for n := 2; seen[name]; n++ {
		name = fmt.Sprintf("%s_%d", base, n)
	}
	seen[name] = true
	return name
}
