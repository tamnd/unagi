package lower

import (
	"go/ast"
	"go/token"
	"sort"
)

// This file keeps the static tier's view of a rebindable module global in step
// with the boxed binding that owns it. A module scalar global a static form reads
// gets a package-level typed shadow and a world-age version counter, declared in
// the assembly seam. Every boxed store to that global is followed by a Rebind that
// reads the object just written and refreshes the pair: the shadow takes the
// native value and the version becomes 1 when the value is exactly the shadow's
// type, or 2 otherwise. A static reader guards the counter at entry and reads the
// shadow only when it is 1, so a rebind the shadow cannot hold, or a delete that
// unbinds the global, routes the read to the boxed twin instead of a stale value.
//
// The shadow and version names must match the ones the ir bridge spells when it
// lowers a tracked read and emits the entry guard. pkg/lower does not import
// pkg/ir, so the two spellings live independently and are kept identical by hand:
// bshadow_<name> and bver_<name>.

// shadowVar names the package-level typed shadow of a tracked global.
func shadowVar(name string) string { return "bshadow_" + name }

// shadowVer names the package-level world-age version counter of a tracked global.
func shadowVer(name string) string { return "bver_" + name }

// shadowGoType maps a tracked global's scalar type to the Go type its shadow
// holds. The four scalar types are the only ones the analysis tracks, so an
// unrecognized type is a build bug rather than a runtime path; it falls back to
// objects.Object, which still compiles and simply never matches a native read.
func shadowGoType(scalar string) string {
	switch scalar {
	case "int":
		return "int64"
	case "float":
		return "float64"
	case "bool":
		return "bool"
	case "str":
		return "string"
	}
	return "objects.Object"
}

// rebindFunc names the runtime helper that refreshes a tracked global's shadow
// and version from a freshly stored object, one per scalar type. Each returns the
// native value and the version the entry guard compares against.
func rebindFunc(scalar string) string {
	switch scalar {
	case "int":
		return "RebindInt"
	case "float":
		return "RebindFloat"
	case "bool":
		return "RebindBool"
	case "str":
		return "RebindStr"
	}
	return "RebindInt"
}

// sortedShadows returns the tracked global names in a stable order, so the shadow
// declarations emit deterministically.
func sortedShadows(tracked map[string]string) []string {
	names := make([]string, 0, len(tracked))
	for n := range tracked {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// bumpShadow appends the world-age update after a boxed store to name, keeping the
// static tier's shadow and version in step with the binding the store just made.
// It fires only when name is a tracked global and the store is a genuine
// module-global write: a module-scope assignment to a module variable, or a
// function assignment to a name that function declared global. A same-named
// function local without a global declaration is a distinct binding and must not
// touch the shadow, so it is filtered out here. The emitted statement reads the
// object the store wrote (the mangled module variable) and assigns the shadow and
// version from the matching Rebind helper.
func (f *fnCtx) bumpShadow(name string) {
	scalar, ok := f.e.tracked[name]
	if !ok {
		return
	}
	moduleWrite := (!f.inFunc && f.e.moduleVars[name]) || (f.inFunc && f.globals[name])
	if !moduleWrite {
		return
	}
	f.add(assign(token.ASSIGN,
		[]ast.Expr{ident(shadowVar(name)), ident(shadowVer(name))},
		callExpr(sel("runtime", rebindFunc(scalar)), ident(mangle(name)))))
}
