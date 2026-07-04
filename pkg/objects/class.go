package objects

import "fmt"

// classObject is a user-defined class, the type object a class statement
// builds. dict holds the names the class body bound, methods and class
// variables alike, in the order they were written so repr and iteration
// stay deterministic. name is __name__ and qual is __qualname__ with the
// module prefix, which is what repr spells. Single inheritance, the MRO,
// and metaclasses land in a later slice; this slice is the flat class.
type classObject struct {
	name  string
	qual  string
	dict  map[string]Object
	order []string
}

func (*classObject) TypeName() string { return "type" }

// instanceObject is an instance of a user class. dict is its __dict__, the
// per-instance attribute store; a name missing here falls back to the class.
type instanceObject struct {
	cls  *classObject
	dict map[string]Object
}

func (o *instanceObject) TypeName() string { return o.cls.name }

// boundMethod pairs a function with the instance it was read from, so a
// later call prepends that instance as self. Reading a method off an
// instance produces one of these; reading it off the class returns the
// plain function.
type boundMethod struct {
	fn   *functionObject
	self Object
}

func (*boundMethod) TypeName() string { return "method" }

// NewClass builds a class object from the names its body bound and their
// values, kept parallel and in body order. The lowering evaluates the
// class body and hands the results here.
func NewClass(name, qual string, names []string, vals []Object) Object {
	c := &classObject{name: name, qual: qual, dict: make(map[string]Object, len(names))}
	for i, n := range names {
		if _, seen := c.dict[n]; !seen {
			c.order = append(c.order, n)
		}
		c.dict[n] = vals[i]
	}
	return c
}

// lookup finds a name on the class. Inheritance would widen this to the MRO;
// for now it is the class's own dict.
func (c *classObject) lookup(name string) (Object, bool) {
	v, ok := c.dict[name]
	return v, ok
}

// setAttr stores a class attribute, tracking insertion order for names that
// are new.
func (c *classObject) setAttr(name string, v Object) {
	if _, seen := c.dict[name]; !seen {
		c.order = append(c.order, name)
	}
	c.dict[name] = v
}

// Instantiate builds an instance of a class and runs __init__. The binding
// errors come from the __init__ function object, so they spell C.__init__()
// exactly as a direct call would; a class with no __init__ rejects any
// argument with the takes-no-arguments message probed on 3.14.
func Instantiate(c *classObject, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	inst := &instanceObject{cls: c, dict: map[string]Object{}}
	init, ok := c.lookup("__init__")
	if !ok {
		if len(pos) > 0 || len(kwNames) > 0 {
			return nil, Raise(TypeError, "%s() takes no arguments", c.name)
		}
		return inst, nil
	}
	fn, ok := init.(*functionObject)
	if !ok {
		// A non-function __init__ is legal Python but needs the descriptor
		// protocol to call; that waits on a later slice.
		return nil, Raise(TypeError, "%s() argument after __init__ is not a plain function", c.name)
	}
	withSelf := append([]Object{inst}, pos...)
	ret, err := fn.bind(withSelf, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	if _, isNone := ret.(*noneObject); !isNone {
		return nil, Raise(TypeError, "__init__() should return None, not '%s'", ret.TypeName())
	}
	return inst, nil
}

// LoadAttr reads o.name as a value. On an instance the instance dict wins,
// then a class function binds to the instance and a class variable comes
// back as is; on a class the name comes straight from the class dict. The
// two AttributeError wordings, instance and type object, are probed on 3.14.
func LoadAttr(o Object, name string) (Object, error) {
	switch x := o.(type) {
	case *instanceObject:
		if v, ok := x.dict[name]; ok {
			return v, nil
		}
		if v, ok := x.cls.lookup(name); ok {
			if fn, ok := v.(*functionObject); ok {
				return &boundMethod{fn: fn, self: x}, nil
			}
			return v, nil
		}
		return nil, Raise(AttributeError, "'%s' object has no attribute '%s'", x.cls.name, name)
	case *classObject:
		if v, ok := x.lookup(name); ok {
			return v, nil
		}
		return nil, Raise(AttributeError, "type object '%s' has no attribute '%s'", x.name, name)
	}
	return nil, Raise(AttributeError, "'%s' object has no attribute '%s'", o.TypeName(), name)
}

// StoreAttr writes o.name = val. Instances and classes take new attributes;
// a builtin has no __dict__, which is the wording 3.14 gives.
func StoreAttr(o Object, name string, val Object) error {
	switch x := o.(type) {
	case *instanceObject:
		x.dict[name] = val
		return nil
	case *classObject:
		x.setAttr(name, val)
		return nil
	}
	return Raise(AttributeError,
		"'%s' object has no attribute '%s' and no __dict__ for setting new attributes",
		o.TypeName(), name)
}

// instanceCallMethod dispatches inst.name(args). An instance-dict entry is
// called as a plain value; a class function binds self; a class value that
// is callable is called without self.
func instanceCallMethod(x *instanceObject, name string, args []Object) (Object, error) {
	if v, ok := x.dict[name]; ok {
		return Call(v, args)
	}
	if v, ok := x.cls.lookup(name); ok {
		if fn, ok := v.(*functionObject); ok {
			return fn.bind(append([]Object{x}, args...), nil, nil)
		}
		return Call(v, args)
	}
	return nil, Raise(AttributeError, "'%s' object has no attribute '%s'", x.cls.name, name)
}

// classCallMethod dispatches Cls.name(args): the name resolves on the class
// and is called with the arguments as given, so a method reached this way
// takes its self explicitly.
func classCallMethod(x *classObject, name string, args []Object) (Object, error) {
	if v, ok := x.lookup(name); ok {
		return Call(v, args)
	}
	return nil, Raise(AttributeError, "type object '%s' has no attribute '%s'", x.name, name)
}

// classRepr and instanceRepr match 3.14: a class prints its qualified name,
// an instance prints its class and identity. The address is not stable, so
// fixtures never print a bare instance, the same rule the function reprs
// already follow.
func classRepr(c *classObject) string {
	return fmt.Sprintf("<class '%s'>", c.qual)
}

func instanceRepr(o *instanceObject) string {
	return fmt.Sprintf("<%s object at %p>", o.cls.qual, o)
}

func boundMethodRepr(m *boundMethod) string {
	return fmt.Sprintf("<bound method %s of %s>", m.fn.qual, Repr(m.self))
}
