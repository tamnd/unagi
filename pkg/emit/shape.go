package emit

import (
	"go/ast"
	"go/token"
)

// This file emits a fixed-shape class's Go struct type, the doc 11 tier 2 layout
// the static tier gives instances of a __slots__ class. A boxed instance is an
// objects.Object the attribute path reads through a slot lookup; a static
// instance is a flat Go struct the attribute path reads through a plain field
// load. The struct declared here is that layout: one field per slot, named and
// typed by the slot's scalar representation, in slot order. A static form typed
// against the class names this type, and a field load resolves against these
// fields, so the declaration is what gives both a real Go type to reference.

// EmitShape renders one shape's Go struct type declaration. The static file
// declares one per module shape class so a form typed against the class, and an
// attribute read that loads one of its fields, have a named type to resolve
// against. A declaration a form does not reference yet is legal Go, so the file
// carries one for every module shape whether or not a form uses it.
func EmitShape(shape Shape) (string, error) {
	return Print(shapeStruct(shape))
}

// shapeStruct builds the struct type declaration for one shape: its fields in
// slot order, each typed by the representation the slot fixes. The field names
// are the slot names, so an attribute read on the class lowers to a field load
// by the same name.
func shapeStruct(shape Shape) *ast.GenDecl {
	fields := make([]*ast.Field, len(shape.Fields))
	for i, f := range shape.Fields {
		fields[i] = field(f.Repr.goType(), f.Name)
	}
	return &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{&ast.TypeSpec{
			Name: ident(shape.Name),
			Type: &ast.StructType{Fields: fieldList(fields...)},
		}},
	}
}
