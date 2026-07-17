package ir

import (
	"github.com/tamnd/unagi/pkg/emit"
	"github.com/tamnd/unagi/pkg/frontend"
)

// This file builds the GlobalResolver the bridge lowers a tracked module-global
// read against, and the table of module scalar globals it draws from. The build
// and the partitioner both lower functions that may read a module global, and
// both must hand the bridge the same resolver so a function proven static during
// partitioning lowers the same way when the build emits it. Sharing the
// construction here keeps the two in step.

// TrackedGlobals maps each module scalar global to its scalar type ("int",
// "float", "bool", "str"). It is the whole-module table both consumers derive a
// per-function resolver from; a nil result means the module has no global the
// static tier can shadow.
func TrackedGlobals(m *frontend.Module) map[string]string {
	gs := frontend.ScalarGlobals(m)
	if len(gs) == 0 {
		return nil
	}
	out := make(map[string]string, len(gs))
	for _, g := range gs {
		out[g.Name] = g.Type
	}
	return out
}

// GlobalRepr maps a tracked global's scalar type to the doc 04 representation the
// bridge reads it through. It reports false for a type outside the scalar subset,
// which never happens for a name TrackedGlobals returns but keeps the mapping
// total for a caller that passes an arbitrary string.
func GlobalRepr(scalar string) (emit.Repr, bool) {
	switch scalar {
	case "int":
		return emit.Repr{Go: "int64", Scalar: emit.SInt}, true
	case "float":
		return emit.Repr{Go: "float64", Scalar: emit.SFloat}, true
	case "bool":
		return emit.Repr{Go: "bool", Scalar: emit.SBool}, true
	case "str":
		return emit.Repr{Go: "string", Scalar: emit.SStr}, true
	}
	return emit.Repr{}, false
}

// GlobalResolverFor builds the per-function resolver from the whole-module tracked
// table. It accepts a name only when the name is a tracked global and fn does not
// bind it as a local: a name fn assigns without a global declaration is a distinct
// local binding, so reading it before that assignment is an UnboundLocalError the
// boxed tier must raise, never a module-global read. A function that declares the
// name global does not bind it locally, so the resolver accepts it and the read
// reaches the global's shadow. A nil or empty table returns a nil resolver, which
// tracks no global and lowers exactly as the resolver-free bridge did.
func GlobalResolverFor(fn *frontend.FuncDef, tracked map[string]string) GlobalResolver {
	if len(tracked) == 0 {
		return nil
	}
	locals := frontend.LocalBindings(fn)
	return func(name string) (emit.Repr, bool) {
		if locals[name] {
			return emit.Repr{}, false
		}
		scalar, ok := tracked[name]
		if !ok {
			return emit.Repr{}, false
		}
		return GlobalRepr(scalar)
	}
}
