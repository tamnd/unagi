package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _ast is the AST accelerator CPython implements in C, the module ast.py opens
// with `from _ast import *`. There is no pure-Python fallback, so ast and its
// dependents (annotationlib, and through it inspect, dataclasses, traceback,
// unittest, logging) cannot import until this exists.
//
// The module ships two things: the Python AST node type hierarchy and the
// compile() mode flags. The node hierarchy is a fixed grammar (Parser/Python.asdl):
// a root AST, nineteen abstract sum-type bases that derive directly from it, and
// the concrete node classes under those bases. Every class carries a _fields
// tuple naming its child slots; the abstract bases carry the _attributes tuple of
// source-position slots, which the leaves inherit.
//
// What the module does not carry under AOT is a parser. ast.parse compiles source
// with compile(..., PyCF_ONLY_AST), and the compiled world has no compile(); there
// is no runtime Python parser to build a real tree. So _ast exposes the node
// surface and the flags, the same reduced-surface stance marshal takes under AOT.
// The consumers on our import path call ast.parse only lazily inside functions,
// never at import, so the node surface is enough to unblock the chain.

func init() {
	moduleTable["_ast"] = &moduleEntry{builtin: true, exec: initAst}
}

// astNodeDef is one row of the node grammar: the class name, the name of its
// direct base (empty for the AST root), its own _fields, and its own _attributes.
// setAttrs records whether the class defines _attributes itself; a leaf inherits
// _attributes from its abstract base and leaves it unset. The table is generated
// by introspecting the oracle CPython 3.14.6 _ast, the interpreter the conformance
// harness runs, so it is platform-stable by construction, and it is ordered
// topologically so a base is always built before the classes that derive from it.
type astNodeDef struct {
	name     string
	base     string
	fields   []string
	attrs    []string
	setAttrs bool
}

// astPyCFFlags are the compile() mode flags _ast exports as plain ints. ast.parse
// passes PyCF_ONLY_AST to compile; the others tune what the parser records. They
// are lazy on our import path but cost nothing to expose.
var astPyCFFlags = []struct {
	name string
	val  int64
}{
	{"PyCF_ONLY_AST", 1024},
	{"PyCF_TYPE_COMMENTS", 4096},
	{"PyCF_ALLOW_TOP_LEVEL_AWAIT", 8192},
	{"PyCF_OPTIMIZED_AST", 33792},
}

func initAst(m *objects.Module) error {
	astInit := objects.NewFuncKwT("__init__", astNodeInit)
	classes := make(map[string]objects.Object, len(astNodes))
	for _, d := range astNodes {
		names := []string{"_fields"}
		vals := []objects.Object{objects.NewTuple(strTuple(d.fields))}
		if d.setAttrs {
			names = append(names, "_attributes")
			vals = append(vals, objects.NewTuple(strTuple(d.attrs)))
		}
		var bases []objects.Object
		if d.base == "" {
			// The AST root carries the shared generic constructor every node
			// inherits; it takes no base of its own.
			names = append(names, "__init__")
			vals = append(vals, astInit)
		} else {
			bases = []objects.Object{classes[d.base]}
		}
		c, err := objects.NewClass(d.name, d.name, bases, names, vals, nil, nil)
		if err != nil {
			return err
		}
		classes[d.name] = c
		if err := objects.StoreAttr(m, d.name, c); err != nil {
			return err
		}
	}
	for _, f := range astPyCFFlags {
		if err := objects.StoreAttr(m, f.name, objects.NewInt(f.val)); err != nil {
			return err
		}
	}
	return nil
}

// astNodeInit is the generic AST.__init__ every node class inherits, mirroring
// CPython's C-level constructor: it reads self._fields, binds positional arguments
// to those field names in order, then applies keyword arguments by name. Reading
// _fields off self at call time rather than capturing it per class is what keeps a
// Python subclass of a node constructing against its own fields. Excess positional
// arguments raise TypeError the way CPython does.
func astNodeInit(t *objects.Thread, pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) == 0 {
		return nil, objects.Raise(objects.TypeError, "__init__() missing self argument")
	}
	self := pos[0]
	args := pos[1:]
	fieldsObj, err := objects.LoadAttr(self, "_fields")
	if err != nil {
		return nil, err
	}
	it, err := objects.Iter(fieldsObj)
	if err != nil {
		return nil, err
	}
	var fields []string
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		s, ok := objects.AsStr(v)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "_fields must contain only str")
		}
		fields = append(fields, s)
	}
	if len(args) > len(fields) {
		plural := ""
		if len(fields) != 1 {
			plural = "s"
		}
		return nil, objects.Raise(objects.TypeError, "%s constructor takes at most %d positional argument%s", self.TypeName(), len(fields), plural)
	}
	for i, a := range args {
		if err := objects.StoreAttr(self, fields[i], a); err != nil {
			return nil, err
		}
	}
	for i, name := range kwNames {
		if err := objects.StoreAttr(self, name, kwVals[i]); err != nil {
			return nil, err
		}
	}
	return objects.None, nil
}
