package ir

import (
	"github.com/tamnd/unagi/pkg/emit"
	"github.com/tamnd/unagi/pkg/frontend"
)

// This file builds the ShapeResolver the bridge lowers a fixed-shape class
// parameter against, and the table of module shape classes it draws from. Like
// the tracked-global resolver next to it, the partitioner and the build both
// lower functions that may take a shape parameter, and both must hand the bridge
// the same resolver so a function proven static during partitioning lowers the
// same way when the build emits it. Sharing the construction here keeps the two
// in step.

// TrackedShapes maps each module fixed-shape class to the doc 04 struct
// representation the bridge types a parameter of that class as. It is the
// whole-module table both consumers derive a resolver from; a nil result means
// the module has no class whose instances the static tier can lower to a struct.
func TrackedShapes(m *frontend.Module) map[string]emit.Repr {
	cs := frontend.ShapeClasses(m)
	if len(cs) == 0 {
		return nil
	}
	out := make(map[string]emit.Repr, len(cs))
	for _, c := range cs {
		out[c.Name] = shapeRepr(c)
	}
	return out
}

// shapeRepr builds the struct representation of one shape class: the Go struct
// type name is the class name, and each field carries the scalar representation
// its slot type fixes, in slot order. The field types come from the same scalar
// mapping the tracked-global resolver uses, so a slot and a global of the same
// type read through the identical Go type.
func shapeRepr(c frontend.ShapeClass) emit.Repr {
	fields := make([]emit.ShapeField, len(c.Fields))
	for i, f := range c.Fields {
		r, _ := GlobalRepr(f.Type)
		fields[i] = emit.ShapeField{Name: f.Name, Repr: r}
	}
	return emit.Repr{Go: c.Name, Shape: &emit.Shape{Name: c.Name, Fields: fields}}
}

// ShapeResolverFor builds the resolver the bridge queries for a class parameter's
// representation from the whole-module shape table. It accepts a name only when
// the table holds it as a fixed-shape class; every other annotation reports false
// and keeps its parameter boxed. A nil or empty table returns a nil resolver,
// which resolves no shape and lowers exactly as the resolver-free bridge did.
//
// Unlike a global read, a class-parameter annotation is not shadowed by a
// function local: the shape analysis already requires the class name to be bound
// exactly once across the whole module, so one module-wide resolver serves every
// function.
func ShapeResolverFor(tracked map[string]emit.Repr) ShapeResolver {
	if len(tracked) == 0 {
		return nil
	}
	return func(name string) (emit.Repr, bool) {
		r, ok := tracked[name]
		return r, ok
	}
}
