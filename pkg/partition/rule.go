// Package partition decides, for every partition unit, whether it compiles to
// the native unboxed static tier or runs on the boxed object model, per doc 06.
// This file is the disqualifier catalog: the normative list of rules from doc 06
// sections 4 and 5, each as a registered record the census and the decision core
// key off, and the same table doc 06 section 17.4 requires so the compiler, the
// tests, and the build report can never disagree about what a rule id means.
package partition

import (
	"fmt"
	"sort"
)

// Scope is how far a rule's demotion reaches when it fires, from doc 06 section
// 4, which names the scope of every rule.
type Scope uint8

const (
	// ScopeUnit demotes just the unit that trips the rule.
	ScopeUnit Scope = iota
	// ScopeRegion demotes only an operation or region into a boxed excursion,
	// leaving the surrounding unit static.
	ScopeRegion
	// ScopeClass opens a class's layout, boxing its instances everywhere and
	// re-planning every unit that depended on its struct layout.
	ScopeClass
	// ScopeBinding marks a module binding rebindable, which a binding guard
	// covers rather than a demotion.
	ScopeBinding
	// ScopeModule poisons a whole module's namespace, routing every cross-module
	// read through the boxed lookup path.
	ScopeModule
	// ScopeProgram demotes the tripping unit and every unit whose state depended
	// on the fact, transitively.
	ScopeProgram
)

// String names the scope for the build report.
func (s Scope) String() string {
	switch s {
	case ScopeUnit:
		return "unit"
	case ScopeRegion:
		return "region"
	case ScopeClass:
		return "class"
	case ScopeBinding:
		return "binding"
	case ScopeModule:
		return "module"
	case ScopeProgram:
		return "program"
	}
	return "unknown"
}

// Rule is one entry in the disqualifier catalog: a stable id, the doc 06 section
// it comes from, the scope its demotion reaches, whether a guard can rescue the
// assumption, whether it is a hard census-time disqualifier or a soft
// type-adequacy or cost verdict, and one sentence of prose for the report. Ids
// are append-only per doc 06 section 10.4: a rule may be retired but an id is
// never reused with a different meaning.
type Rule struct {
	ID        string
	Section   string
	Scope     Scope
	Guardable bool
	Hard      bool
	Prose     string
}

// The rule ids the spec names directly are kept verbatim so the report wording
// in doc 06 sections 10.3 and 15 matches the source. The rest follow the same
// kebab convention.
const (
	RuleEvalDynamicSource     = "eval-dynamic-source"
	RuleExecDynamicSource     = "exec-dynamic-source"
	RuleCompileDynamicSource  = "compile-dynamic-source"
	RuleSetattrDynamic        = "setattr-dynamic"
	RuleDictWriteDynamic      = "dict-write-dynamic"
	RuleVarsMutation          = "vars-mutation"
	RuleCrossModuleRebind     = "cross-module-rebind"
	RuleCrossModuleRebindWild = "cross-module-rebind-dynamic"
	RuleMetaclassOpaque       = "metaclass-opaque"
	RuleDynamicClassConstruct = "dynamic-class-construction"
	RuleClassDecoratorOpaque  = "class-decorator-opaque"
	RuleClassReassignment     = "class-reassignment"
	RuleFrameWalkerDirect     = "frame-walker-direct"
	RuleLocalsCall            = "locals-call"
	RuleFrameWalkerCaller     = "frame-walker-caller"
	RuleDynamicImport         = "dynamic-import"
	RuleAnyWidth              = "any-width"
	RuleDelPossiblyUnbound    = "del-possibly-unbound"
	RuleStarargsUnknownCallee = "starargs-unknown-callee"
	RuleGetattrHook           = "getattr-hook"
	RuleSetattrHook           = "setattr-hook"
	RuleGetattributeHook      = "getattribute-hook"
	RuleDescriptorUnmodeled   = "descriptor-unmodeled"
	RuleInheritBoxedBase      = "inherit-boxed-base"
	RuleDecoratorOpaque       = "decorator-opaque"
	RuleExcursionBudget       = "excursion-budget-exceeded"
	RuleCostModel             = "cost-model-verdict"
	RuleGuardBudget           = "guard-budget-exceeded"
)

// catalog is the one registered table of every rule. It is built once at init
// and never mutated, so lookups are read-only and the CI coverage invariant of
// doc 06 section 17.4 has a single source of truth.
var catalog = func() map[string]Rule {
	rules := []Rule{
		// 4.1 eval, exec, and compile of non-constant source.
		{RuleEvalDynamicSource, "4.1", ScopeProgram, false, true,
			"eval on a non-constant string can read and write the caller's namespace"},
		{RuleExecDynamicSource, "4.1", ScopeProgram, false, true,
			"exec on a non-constant string can mutate locals no static analysis survives"},
		{RuleCompileDynamicSource, "4.1", ScopeUnit, false, true,
			"compile of a non-constant string builds code the analysis cannot see"},
		// 4.2 dynamic attribute mutation on statically-typed objects.
		{RuleSetattrDynamic, "4.2", ScopeClass, false, true,
			"setattr with a computed name opens the target class's layout"},
		{RuleDictWriteDynamic, "4.2", ScopeClass, false, true,
			"a direct __dict__ write opens the target class's layout"},
		{RuleVarsMutation, "4.2", ScopeClass, false, true,
			"vars(obj) mutation opens the target class's layout"},
		// 4.3 cross-module monkeypatching.
		{RuleCrossModuleRebind, "4.3", ScopeBinding, true, false,
			"a store to another module's attribute makes the binding rebindable, covered by a binding guard"},
		{RuleCrossModuleRebindWild, "4.3", ScopeModule, false, true,
			"a store through a computed module attribute name poisons the module's namespace"},
		// 4.4 metaclass and dynamic class construction.
		{RuleMetaclassOpaque, "4.4", ScopeClass, false, true,
			"a metaclass outside the analyzable allow list is opaque, so instances are boxed"},
		{RuleDynamicClassConstruct, "4.4", ScopeClass, false, true,
			"three-argument type(name, bases, ns) builds a class the analysis cannot lay out"},
		{RuleClassDecoratorOpaque, "4.4", ScopeClass, false, true,
			"a class decorator that returns a different object than it received is opaque"},
		{RuleClassReassignment, "15", ScopeClass, false, true,
			"assigning obj.__class__ opens both classes involved"},
		// 4.5 frame and namespace introspection.
		{RuleFrameWalkerDirect, "4.5", ScopeUnit, false, true,
			"sys._getframe or inspect.currentframe observes the live frame, which the static tier has no dictionary for"},
		{RuleLocalsCall, "4.5", ScopeUnit, false, true,
			"locals() has no dictionary to return when locals are Go variables"},
		{RuleFrameWalkerCaller, "4.5", ScopeProgram, false, false,
			"a caller of a frame-walker is observable, so it emits synthetic-frame tables"},
		// 4.6 dynamic import.
		{RuleDynamicImport, "4.6", ScopeRegion, false, false,
			"a non-constant import becomes a boxed excursion yielding a boxed module object"},
		// 4.7 Any and the width of speculation.
		{RuleAnyWidth, "4.7", ScopeRegion, true, false,
			"an Any with no usable hint flows into boxed excursions operation by operation"},
		// 4.8 the remaining census entries.
		{RuleDelPossiblyUnbound, "4.8", ScopeUnit, false, true,
			"a possibly-unbound read after del needs the boxed frame's unbound tracking"},
		{RuleStarargsUnknownCallee, "4.8", ScopeRegion, false, false,
			"f(*args, **kwargs) on an untyped callee is a boxed excursion at the call site"},
		{RuleGetattrHook, "4.8", ScopeClass, false, true,
			"__getattr__ or __getattribute__ on a class means instances cannot honor a static layout"},
		{RuleSetattrHook, "4.8", ScopeClass, false, true,
			"__setattr__ on a class means a static layout cannot honor per-access hooks"},
		{RuleGetattributeHook, "4.8", ScopeClass, false, true,
			"__getattribute__ on a class intercepts every access, so instances are boxed"},
		{RuleDescriptorUnmodeled, "4.8", ScopeRegion, false, false,
			"an unmodeled side-effecting descriptor lowers that access to the boxed descriptor path"},
		{RuleInheritBoxedBase, "4.8", ScopeClass, false, true,
			"a subclass of a boxed class is boxed, since struct embedding cannot extend a compact-dict instance"},
		{RuleDecoratorOpaque, "15", ScopeUnit, false, false,
			"an unmodeled decorator binds the name to a boxed callable; the body may still be static behind it"},
		// 5.6 and 5.7 the soft cost verdicts.
		{RuleExcursionBudget, "5.6", ScopeUnit, false, false,
			"the unit's boxed excursions exceed the 25 percent budget"},
		{RuleCostModel, "5.7", ScopeUnit, false, false,
			"the static score is not below 60 percent of the boxed score, so the static form is not worth emitting"},
		// 7.6 the guard budget (planned in slice 6, id registered here).
		{RuleGuardBudget, "7.6", ScopeUnit, false, false,
			"the planned guards exceed 15 percent of the static score, so the unit spends its time checking assumptions"},
	}
	m := make(map[string]Rule, len(rules))
	for _, r := range rules {
		if _, dup := m[r.ID]; dup {
			panic(fmt.Sprintf("partition: duplicate rule id %q", r.ID))
		}
		m[r.ID] = r
	}
	return m
}()

// LookupRule returns the catalog entry for an id and whether it exists. A miss
// is a programming error at a fire site, never a runtime condition.
func LookupRule(id string) (Rule, bool) {
	r, ok := catalog[id]
	return r, ok
}

// MustRule returns the catalog entry for an id, panicking on a miss so a typo in
// a fire site fails loudly at the first test rather than emitting a report with
// a meaningless reason.
func MustRule(id string) Rule {
	r, ok := catalog[id]
	if !ok {
		panic(fmt.Sprintf("partition: unknown rule id %q", id))
	}
	return r
}

// AllRules returns every registered rule in id order, the table doc 06 section
// 17.4's coverage invariant iterates and the build report's --by-reason
// aggregation groups on.
func AllRules() []Rule {
	out := make([]Rule, 0, len(catalog))
	for _, r := range catalog {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
