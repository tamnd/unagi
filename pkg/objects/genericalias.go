package objects

import "strings"

// genericAliasObject is a parameterized builtin generic, the value list[int] and
// dict[str, int] evaluate to. types.GenericAlias produces it, and subscripting a
// builtin container type (list[int]) is the sugar for the same thing. It carries
// the origin type and the argument tuple, reprs as `list[int]`, hashes and
// compares by origin plus args, and calling it constructs the origin, so
// list[int]() builds a list. _collections_abc opens with
// `GenericAlias = type(list[int])` and hangs __class_getitem__ = classmethod(
// GenericAlias) off its ABCs, so this is what makes that module import.
type genericAliasObject struct {
	origin Object
	args   []Object
}

func (*genericAliasObject) TypeName() string { return "types.GenericAlias" }

// NewGenericAlias builds origin[item]. A tuple item spreads into the argument
// list, so list[int] carries one argument and dict[str, int] carries two; any
// other item is a single argument. This matches types.GenericAlias, which
// normalizes its second argument to a tuple for __args__.
func NewGenericAlias(origin, item Object) Object {
	var args []Object
	if tup, ok := item.(*tupleObject); ok {
		args = append(args, tup.elts...)
	} else {
		args = []Object{item}
	}
	return &genericAliasObject{origin: origin, args: args}
}

// genericAliasLoadAttr answers the three attributes a GenericAlias exposes:
// __origin__ is the parameterized type, __args__ the argument tuple, and
// __parameters__ the type variables among the arguments, empty here since the
// floor never parameterizes with a TypeVar.
func genericAliasLoadAttr(g *genericAliasObject, name string) (Object, error) {
	switch name {
	case "__origin__":
		return g.origin, nil
	case "__args__":
		return NewTuple(append([]Object(nil), g.args...)), nil
	case "__parameters__":
		return NewTuple(nil), nil
	}
	return nil, Raise(AttributeError, "'types.GenericAlias' object has no attribute '%s'", name)
}

// genericAliasRepr renders origin[arg, arg] the way CPython does: the origin and
// each argument print through typeReprForAlias, so a type prints as its bare
// qualname and Ellipsis as `...`.
func genericAliasRepr(g *genericAliasObject) (string, error) {
	head, err := typeReprForAlias(g.origin)
	if err != nil {
		return "", err
	}
	parts := make([]string, len(g.args))
	for i, a := range g.args {
		s, err := typeReprForAlias(a)
		if err != nil {
			return "", err
		}
		parts[i] = s
	}
	return head + "[" + strings.Join(parts, ", ") + "]", nil
}

// typeReprForAlias is CPython's _type_repr: a type shows its qualname, Ellipsis
// shows as `...`, a nested alias shows its own repr, and anything else falls back
// to repr(). It keeps list[int] from printing as <class 'list'>[<class 'int'>].
func typeReprForAlias(o Object) (string, error) {
	switch x := o.(type) {
	case *ellipsisObject:
		return "...", nil
	case *genericAliasObject:
		return genericAliasRepr(x)
	case *classObject:
		return x.qual, nil
	case *typeObject:
		return x.name, nil
	}
	if name, ok := BuiltinFuncName(o); ok && builtinTypeReprs[name] {
		return name, nil
	}
	return ReprE(o)
}

// subscriptableBuiltinType names the builtin container types that answer a
// subscript with a GenericAlias, mirroring the __class_getitem__ CPython defines
// on them. len[int] and int[str] stay errors, since those builtins have no such
// hook.
var subscriptableBuiltinType = map[string]bool{
	"list": true, "dict": true, "tuple": true,
	"set": true, "frozenset": true, "type": true,
}
