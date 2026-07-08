package partition

import (
	"fmt"
	"sort"

	"github.com/tamnd/unagi/pkg/types"
)

// This file is the census, phase one of doc 06 section 3.2. It walks nothing on
// its own; the IR pass that owns the walk records facts into it, and the census
// holds them plus the whole-program side tables the later phases read: which
// classes a dynamic store has opened, which modules a computed-name store has
// poisoned, and which module bindings are rebindable. The census is purely
// fact-collection, stable against everything the type and cost phases do, and
// part of the determinism hash of section 9.

// Unit identifies one partition unit: a function, comprehension, or class body.
// Module and Offset give the deterministic processing order doc 06 section 9.2
// requires; Name and Span drive the report.
type Unit struct {
	Module string
	Name   string
	Span   types.Span
	Offset int
}

// Key is a stable identity for a unit within a program: module, name, and source
// offset, which distinguishes two same-named nested units.
func (u Unit) Key() string {
	return fmt.Sprintf("%s\x00%s\x00%d", u.Module, u.Name, u.Offset)
}

// String renders the unit as the report's header form, qualified name then span.
func (u Unit) String() string {
	return fmt.Sprintf("%s.%s  %s", u.Module, u.Name, u.Span)
}

// Fact is one census entry: the rule that fired, where, and the scoped target it
// names (a class, module, or binding) when the rule's scope needs one. Target is
// empty for unit- and region-scoped rules.
type Fact struct {
	Rule   string
	Span   types.Span
	Target string
}

// openReason records why a class's layout opened, for the report.
type openReason struct {
	Rule string
	Span types.Span
}

// Census holds the recorded facts and the whole-program side tables. It is built
// empty and filled by Record; nothing here consults types.
type Census struct {
	facts   map[string][]Fact
	units   map[string]Unit
	opened  map[string]openReason // class name -> why it opened
	poison  map[string]openReason // module name -> why its namespace is poisoned
	rebind  map[string]bool       // module binding name -> rebindable
	walkers map[string]bool       // unit key -> calls a frame-walker directly
}

// NewCensus returns an empty census.
func NewCensus() *Census {
	return &Census{
		facts:   map[string][]Fact{},
		units:   map[string]Unit{},
		opened:  map[string]openReason{},
		poison:  map[string]openReason{},
		rebind:  map[string]bool{},
		walkers: map[string]bool{},
	}
}

// Record files a fact against a unit, updating the side tables the fact's scope
// implies. The rule id must be in the catalog; an unknown id is a fire-site bug
// and panics rather than emitting a report with a meaningless reason. Class-scope
// facts open their target class (monotone: a class never re-closes), module-scope
// facts poison their target module, binding-scope facts mark the binding
// rebindable, and a direct frame-walker fact marks the unit for the caller
// transitivity of section 4.5.
func (c *Census) Record(u Unit, f Fact) {
	rule := MustRule(f.Rule)
	c.units[u.Key()] = u
	c.facts[u.Key()] = append(c.facts[u.Key()], f)

	switch rule.Scope {
	case ScopeClass:
		if f.Target != "" {
			if _, already := c.opened[f.Target]; !already {
				c.opened[f.Target] = openReason{Rule: f.Rule, Span: f.Span}
			}
		}
	case ScopeModule:
		if f.Target != "" {
			if _, already := c.poison[f.Target]; !already {
				c.poison[f.Target] = openReason{Rule: f.Rule, Span: f.Span}
			}
		}
	case ScopeBinding:
		if f.Target != "" {
			c.rebind[f.Target] = true
		}
	}
	if f.Rule == RuleFrameWalkerDirect || f.Rule == RuleLocalsCall {
		c.walkers[u.Key()] = true
	}
}

// Facts returns the facts recorded against a unit, in record order.
func (c *Census) Facts(u Unit) []Fact { return c.facts[u.Key()] }

// ClassClosed reports whether a class is still layout-closed, the section 5.3
// query the type-adequacy phase uses to decide whether instances may be unboxed.
// A class with no opening store is closed; classes start optimistically closed.
func (c *Census) ClassClosed(class string) bool {
	_, opened := c.opened[class]
	return !opened
}

// ClassOpenedBy returns the rule and span that opened a class, for the report's
// suggestion line, and whether the class is open at all.
func (c *Census) ClassOpenedBy(class string) (string, types.Span, bool) {
	r, ok := c.opened[class]
	return r.Rule, r.Span, ok
}

// ModulePoisoned reports whether a module's namespace is poisoned, so every
// cross-module read of it takes the boxed lookup path.
func (c *Census) ModulePoisoned(mod string) bool {
	_, bad := c.poison[mod]
	return bad
}

// BindingRebindable reports whether a module binding is stored to anywhere, so a
// static read of it needs a binding guard rather than a direct load.
func (c *Census) BindingRebindable(name string) bool { return c.rebind[name] }

// CallsFrameWalker reports whether a unit calls a frame-walker or locals()
// directly, the mark section 4.5 propagates to callers.
func (c *Census) CallsFrameWalker(u Unit) bool { return c.walkers[u.Key()] }

// Units returns every unit the census has seen, in the deterministic processing
// order of doc 06 section 9.2: sorted by module path then source offset, with
// name and key as final tie-breakers so the order is total.
func (c *Census) Units() []Unit {
	out := make([]Unit, 0, len(c.units))
	for _, u := range c.units {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Module != b.Module {
			return a.Module < b.Module
		}
		if a.Offset != b.Offset {
			return a.Offset < b.Offset
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Key() < b.Key()
	})
	return out
}
