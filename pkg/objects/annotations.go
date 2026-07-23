package objects

// A class annotation dict is the mapping a class body accumulates for its PEP 526
// variable annotations. Without `from __future__ import annotations` the class
// body evaluates each annotation as it runs and stores it under the annotated
// name, so `class C: x: int` leaves `C.__annotations__ == {'x': int}`. The boxed
// tier evaluates eagerly the same way; forward references that name a not-yet-
// defined global raise at class-definition time, the documented cost of eager
// evaluation until lazy __annotate__ lands.
//
// The dict lives in the class namespace, so it folds into the class dict at
// Finish and reads back through both `C.__annotations__` and
// `C.__dict__['__annotations__']`, the two spellings dataclasses and annotationlib
// read fields through.

// classAnnotations returns a class's __annotations__ mapping: the dict the class
// body accumulated, or a fresh empty dict a class that declared none gets on
// first read and keeps thereafter, so repeated reads hand back the same object
// the way 3.14 memoizes it. It matches CPython in never raising for a class, so
// `C.__annotations__` and `getattr(C, '__annotations__', None)` both give a dict.
func classAnnotations(c *classObject) Object {
	if c.annotations == nil {
		c.annotations = newAttrs()
	}
	return c.annotations
}

// annotationsDescriptor is the getset descriptor CPython installs as
// type.__dict__['__annotations__']. annotationlib binds its __get__ once as
// `_BASE_GET_ANNOTATIONS = type.__dict__["__annotations__"].__get__` and calls it
// to read a class's annotations, falling back to AttributeError for a static
// (builtin) type that carries no annotation dict.
type annotationsDescriptor struct{}

func (*annotationsDescriptor) TypeName() string { return "getset_descriptor" }

// typeAnnotationsDescriptor is the single shared instance stored in type.__dict__.
var typeAnnotationsDescriptor = &annotationsDescriptor{}

// annotationsDescriptorGet is the descriptor's __get__: it hands back the owner
// class's annotation dict, or raises AttributeError for a builtin type, the way
// the getset descriptor does for a static type with no annotation storage. The
// owner argument (the type) is accepted and ignored, matching __get__'s
// (instance, owner=None) shape.
func annotationsDescriptorGet(args []Object) (Object, error) {
	if len(args) < 1 {
		return nil, Raise(TypeError, "__get__() missing required argument")
	}
	c, ok := args[0].(*classObject)
	if !ok {
		return nil, Raise(AttributeError, "__annotations__")
	}
	return classAnnotations(c), nil
}
