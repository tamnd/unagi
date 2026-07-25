package objects

import "strings"

// simpleNamespaceObject is a types.SimpleNamespace: a plain attribute bag with a
// namespace(...) repr. Its attributes live in a real dict so ns.__dict__ is the
// live mapping CPython exposes and the attributes keep first-assignment order,
// which is the order the repr and __dict__ iteration report on 3.14. sys builds
// sys.implementation as one of these.
type simpleNamespaceObject struct {
	dict *dictObject
}

func (*simpleNamespaceObject) TypeName() string { return "types.SimpleNamespace" }

// NewSimpleNamespace builds a namespace whose attributes are the given name and
// value pairs, in order. It is the Go-side constructor sys.implementation and
// any other native builder uses; the Python types.SimpleNamespace(...) call
// routes through newSimpleNamespace.
func NewSimpleNamespace(names []string, vals []Object) Object {
	d := newAttrs()
	for i, n := range names {
		_ = d.set(NewStr(n), vals[i])
	}
	return &simpleNamespaceObject{dict: d}
}

// newSimpleNamespace is the types.SimpleNamespace(...) constructor. It accepts at
// most one positional argument, a mapping whose items seed the namespace, then
// the keyword arguments as attributes, matching CPython 3.14 where the positional
// form was added.
func newSimpleNamespace(pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if len(pos) > 1 {
		return nil, Raise(TypeError, "SimpleNamespace expected at most 1 argument, got %d", len(pos))
	}
	d := newAttrs()
	if len(pos) == 1 {
		src, ok := pos[0].(*dictObject)
		if !ok {
			return nil, Raise(TypeError, "SimpleNamespace() argument must be a mapping, not %s", pos[0].TypeName())
		}
		for _, e := range src.entries {
			if _, ok := e.key.(*strObject); !ok {
				return nil, Raise(TypeError, "keywords must be strings")
			}
			if err := d.set(e.key, e.val); err != nil {
				return nil, err
			}
		}
	}
	for i, n := range kwNames {
		if err := d.set(NewStr(n), kwVals[i]); err != nil {
			return nil, err
		}
	}
	return &simpleNamespaceObject{dict: d}, nil
}

// simpleNamespaceLoadAttr reads ns.name: __dict__ hands back the live attribute
// dict, any stored attribute reads its value, and a miss is the AttributeError
// CPython spells with the qualified type name.
func simpleNamespaceLoadAttr(ns *simpleNamespaceObject, name string) (Object, error) {
	if name == "__dict__" {
		return ns.dict, nil
	}
	if v, ok, err := ns.dict.lookup(NewStr(name)); err != nil {
		return nil, err
	} else if ok {
		return v, nil
	}
	return nil, Raise(AttributeError, "'types.SimpleNamespace' object has no attribute '%s'", name)
}

// simpleNamespaceRepr spells namespace(name=value, ...) over the attributes in
// insertion order, the shape CPython gives a SimpleNamespace on 3.14.
func simpleNamespaceRepr(ns *simpleNamespaceObject, strict bool) (string, error) {
	var s strings.Builder
	s.WriteString("namespace(")
	for i, e := range ns.dict.entries {
		name, _ := AsStr(e.key)
		if i > 0 {
			s.WriteString(", ")
		}
		v, err := reprCore(e.val, strict)
		if err != nil {
			return "", err
		}
		s.WriteString(name)
		s.WriteByte('=')
		s.WriteString(v)
	}
	s.WriteByte(')')
	return s.String(), nil
}
