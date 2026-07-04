package objects

import "fmt"

// classObject is a user-defined class, the type object a class statement
// builds. dict holds the names the class body bound, methods and class
// variables alike, in the order they were written so repr and iteration
// stay deterministic. name is __name__ and qual is __qualname__ with the
// module prefix, which is what repr spells. bases are the direct base
// classes in written order and mro is the C3 linearization starting with
// the class itself; the implicit object root carries no user names, so it
// is left off the chain. Metaclasses and descriptors land in a later slice.
type classObject struct {
	name  string
	qual  string
	dict  map[string]Object
	order []string
	bases []*classObject
	mro   []*classObject
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

// NewClass builds a class object from its bases and the names its body
// bound. bases are the values of the base expressions in written order,
// with the implicit object base passed as nil so the lowering never has to
// name it; each real base must be a class. The C3 linearization runs here,
// so an inconsistent base order raises the same TypeError CPython does at
// class-creation time.
func NewClass(name, qual string, bases []Object, names []string, vals []Object) (Object, error) {
	c := &classObject{name: name, qual: qual, dict: make(map[string]Object, len(names))}
	for i, n := range names {
		if _, seen := c.dict[n]; !seen {
			c.order = append(c.order, n)
		}
		c.dict[n] = vals[i]
	}
	seen := map[*classObject]bool{}
	for _, b := range bases {
		if b == nil {
			// The implicit object base contributes no user names.
			continue
		}
		bc, ok := b.(*classObject)
		if !ok {
			return nil, Raise(TypeError, "bases must be types")
		}
		if seen[bc] {
			return nil, Raise(TypeError, "duplicate base class %s", bc.name)
		}
		seen[bc] = true
		c.bases = append(c.bases, bc)
	}
	mro, err := c3Linearize(c)
	if err != nil {
		return nil, err
	}
	c.mro = mro
	return c, nil
}

// c3Linearize computes the C3 method resolution order for c from its bases'
// own linearizations. The result starts with c and lists every ancestor
// once; the object root is omitted because it holds no user names. A set of
// bases that cannot be ordered consistently raises CPython's exact
// class-creation TypeError.
func c3Linearize(c *classObject) ([]*classObject, error) {
	if len(c.bases) == 0 {
		return []*classObject{c}, nil
	}
	var seqs [][]*classObject
	for _, b := range c.bases {
		seqs = append(seqs, append([]*classObject(nil), b.mro...))
	}
	seqs = append(seqs, append([]*classObject(nil), c.bases...))
	merged, err := c3Merge(seqs)
	if err != nil {
		return nil, err
	}
	return append([]*classObject{c}, merged...), nil
}

// c3Merge is the merge step of C3: repeatedly take a head that appears in no
// sequence's tail and remove it from every head. When no such head exists
// the order is inconsistent, and the blocked heads name the bases in the
// error the way CPython lists them.
func c3Merge(seqs [][]*classObject) ([]*classObject, error) {
	var res []*classObject
	for {
		var live [][]*classObject
		for _, s := range seqs {
			if len(s) > 0 {
				live = append(live, s)
			}
		}
		if len(live) == 0 {
			return res, nil
		}
		var cand *classObject
		for _, s := range live {
			head := s[0]
			if !inSomeTail(head, live) {
				cand = head
				break
			}
		}
		if cand == nil {
			return nil, Raise(TypeError,
				"Cannot create a consistent method resolution order (MRO) for bases %s",
				blockedNames(live))
		}
		res = append(res, cand)
		for i, s := range live {
			if len(s) > 0 && s[0] == cand {
				live[i] = s[1:]
			}
		}
		seqs = live
	}
}

// inSomeTail reports whether c appears past the head of any live sequence.
func inSomeTail(c *classObject, seqs [][]*classObject) bool {
	for _, s := range seqs {
		for _, x := range s[1:] {
			if x == c {
				return true
			}
		}
	}
	return false
}

// blockedNames lists the distinct heads of the live sequences in first-seen
// order, which is the set CPython names when it cannot build the MRO.
func blockedNames(seqs [][]*classObject) string {
	var names []string
	seen := map[*classObject]bool{}
	for _, s := range seqs {
		h := s[0]
		if !seen[h] {
			seen[h] = true
			names = append(names, h.name)
		}
	}
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}

// lookup finds a name on the class by walking the MRO, so an inherited
// method or class variable resolves to the first class that defines it.
func (c *classObject) lookup(name string) (Object, bool) {
	for _, k := range c.mro {
		if v, ok := k.dict[name]; ok {
			return v, ok
		}
	}
	return nil, false
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
	case *superObject:
		return superLoadAttr(x, name)
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

// instanceCallMethodKw is instanceCallMethod with keyword arguments: an
// instance-dict entry is called through CallKw and a class function binds
// self before the keywords reach its binder.
func instanceCallMethodKw(x *instanceObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if v, ok := x.dict[name]; ok {
		return CallKw(v, pos, kwNames, kwVals)
	}
	if v, ok := x.cls.lookup(name); ok {
		if fn, ok := v.(*functionObject); ok {
			return fn.bind(append([]Object{x}, pos...), kwNames, kwVals)
		}
		return CallKw(v, pos, kwNames, kwVals)
	}
	return nil, Raise(AttributeError, "'%s' object has no attribute '%s'", x.cls.name, name)
}

// classCallMethodKw is classCallMethod with keyword arguments, so a method
// reached through the class passes its self explicitly and the keywords land
// on the function's own binder.
func classCallMethodKw(x *classObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	if v, ok := x.lookup(name); ok {
		return CallKw(v, pos, kwNames, kwVals)
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
