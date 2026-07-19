package objects

import "fmt"

// staticmethodObject wraps a function so that accessing it through an instance
// or a class yields the plain function, with no self or cls prepended. It is a
// non-data descriptor: an instance-dict entry of the same name shadows it.
type staticmethodObject struct{ fn Object }

func (*staticmethodObject) TypeName() string { return "staticmethod" }

// classmethodObject wraps a function so that accessing it binds the class it
// was reached through as the first argument. Through an instance the bound
// class is the instance's type, so a classmethod called on a subclass sees the
// subclass. It is a non-data descriptor.
type classmethodObject struct{ fn Object }

func (*classmethodObject) TypeName() string { return "classmethod" }

// propertyObject is a computed attribute: reading it calls fget with the
// instance, writing calls fset, deleting calls fdel, and any of the three
// absent raises the probed AttributeError. It is a data descriptor, so it wins
// over an instance-dict entry of the same name. setter, getter, and deleter
// return a fresh property carrying the replaced slot, which is what the
// @prop.setter decorator idiom relies on.
type propertyObject struct{ fget, fset, fdel Object }

func (*propertyObject) TypeName() string { return "property" }

// NewStaticMethod, NewClassMethod, and NewProperty build the descriptor
// objects. The lowering reaches them through the builtin singletons below when
// staticmethod, classmethod, or property is used as a decorator or called
// directly.
func NewStaticMethod(fn Object) Object { return &staticmethodObject{fn: fn} }
func NewClassMethod(fn Object) Object  { return &classmethodObject{fn: fn} }
func NewProperty(fget, fset, fdel Object) Object {
	return &propertyObject{fget: fget, fset: fset, fdel: fdel}
}

// The builtin singletons for the three descriptor constructors. They are
// funcObjects so a call site treats them like any other builtin; the arity
// wordings are the ones CPython gives, which the generic funcObject check does
// not match, so each does its own count.
var (
	StaticMethodBuiltin Object = NewFunc("staticmethod", -1, func(args []Object) (Object, error) {
		if len(args) != 1 {
			return nil, Raise(TypeError, "staticmethod expected 1 argument, got %d", len(args))
		}
		return NewStaticMethod(args[0]), nil
	})
	ClassMethodBuiltin Object = NewFunc("classmethod", -1, func(args []Object) (Object, error) {
		if len(args) != 1 {
			return nil, Raise(TypeError, "classmethod expected 1 argument, got %d", len(args))
		}
		return NewClassMethod(args[0]), nil
	})
	PropertyBuiltin Object = NewFunc("property", -1, func(args []Object) (Object, error) {
		if len(args) > 4 {
			return nil, Raise(TypeError, "property() takes at most 4 arguments (%d given)", len(args))
		}
		arg := func(i int) Object {
			if i < len(args) {
				return args[i]
			}
			return nil
		}
		// The fourth argument is the docstring, which this tier does not model.
		return NewProperty(noneToNil(arg(0)), noneToNil(arg(1)), noneToNil(arg(2))), nil
	})
)

// noneToNil folds a None argument to a nil slot so property(None, setter) reads
// as "no getter" the way CPython treats it.
func noneToNil(o Object) Object {
	if o == nil {
		return nil
	}
	if _, ok := o.(*noneObject); ok {
		return nil
	}
	return o
}

// isDataDescriptor reports whether a class-dict value takes priority over an
// instance dict on attribute read. A property always qualifies; a user object
// qualifies when its type defines __set__ or __delete__, the rule CPython uses
// to rank a data descriptor above the instance dict.
func isDataDescriptor(v Object) bool {
	if p, ok := descriptorPayload(v); ok {
		v = p
	}
	switch d := v.(type) {
	case *propertyObject, *memberDescriptor:
		return true
	case *instanceObject:
		if _, ok := d.cls.lookup("__set__"); ok {
			return true
		}
		if _, ok := d.cls.lookup("__delete__"); ok {
			return true
		}
	}
	return false
}

// instanceGet applies the descriptor protocol to a class-dict value v resolved
// for the attribute name on the instance x: a plain function binds x as self, a
// staticmethod comes back bare, a classmethod binds x's type, and a property
// calls its getter. name is only used to spell the no-getter error.
func instanceGet(x *instanceObject, name string, v Object) (Object, error) {
	// A classmethod, staticmethod or property subclass instance carries the
	// wrapped descriptor as its payload; with no user descriptor hook of its own
	// it runs the protocol through that payload, so abc's abstractclassmethod and
	// abstractproperty behave as the builtins they subclass.
	if p, ok := descriptorPayload(v); ok {
		v = p
	}
	switch d := v.(type) {
	case *functionObject:
		return &boundMethod{fn: d, self: x}, nil
	case *funcObject:
		// A builtin string dunder that a class reassigned to another slot binds
		// the instance as self, the way a wrapper_descriptor does, so
		// __str__ = int.__repr__ calls int.__repr__(self). A plain builtin
		// leaves selfBound false and comes back unbound.
		if d.selfBound {
			return bindBuiltinSelf(d, x), nil
		}
	case *staticmethodObject:
		return d.fn, nil
	case *classmethodObject:
		return classmethodBind(d.fn, x.cls), nil
	case *propertyObject:
		if d.fget == nil {
			return nil, Raise(AttributeError, "property '%s' of '%s' object has no getter", name, x.cls.name)
		}
		return Call(d.fget, []Object{x})
	case *cachedPropertyObject:
		return cachedPropertyGet(x, name, d)
	case *memberDescriptor:
		return slotGet(x, d)
	case *instanceObject:
		// A user descriptor with __get__ runs __get__(descr, instance, owner);
		// owner is the instance's type. Without __get__ the object is a plain
		// class attribute and comes back as is.
		if _, ok := d.cls.lookup("__get__"); ok {
			return instanceCallMethod(d, "__get__", []Object{x, x.cls})
		}
	}
	return v, nil
}

// descriptorSubclassAttr resolves an attribute read on a property subclass
// instance by delegating to the wrapped property. getter, setter and deleter
// each return a fresh instance of the same subclass carrying the replaced slot,
// so the @prop.setter chain preserves the subclass the way CPython does, and
// fget, fset and fdel read the stored callables back. ok is false for any other
// instance or name, so the ordinary AttributeError still stands. The classmethod
// and staticmethod subclasses expose no such attributes in this tier.
func descriptorSubclassAttr(x *instanceObject, name string) (Object, bool) {
	payload, ok := descriptorPayload(x)
	if !ok {
		return nil, false
	}
	p, ok := payload.(*propertyObject)
	if !ok {
		return nil, false
	}
	rewrap := func(np *propertyObject) Object {
		return &instanceObject{cls: x.cls, attrs: newAttrs(), builtinData: np}
	}
	slotOrNone := func(o Object) Object {
		if o == nil {
			return None
		}
		return o
	}
	switch name {
	case "getter":
		return NewFunc("getter", 1, func(a []Object) (Object, error) {
			return rewrap(&propertyObject{fget: a[0], fset: p.fset, fdel: p.fdel}), nil
		}), true
	case "setter":
		return NewFunc("setter", 1, func(a []Object) (Object, error) {
			return rewrap(&propertyObject{fget: p.fget, fset: a[0], fdel: p.fdel}), nil
		}), true
	case "deleter":
		return NewFunc("deleter", 1, func(a []Object) (Object, error) {
			return rewrap(&propertyObject{fget: p.fget, fset: p.fset, fdel: a[0]}), nil
		}), true
	case "fget":
		return slotOrNone(p.fget), true
	case "fset":
		return slotOrNone(p.fset), true
	case "fdel":
		return slotOrNone(p.fdel), true
	}
	return nil, false
}

// bindBuiltinSelf wraps a self-bound builtin so a call prepends the instance,
// the way reading a wrapper_descriptor off an instance yields a bound method.
func bindBuiltinSelf(d *funcObject, self Object) Object {
	return NewFunc(d.name, -1, func(args []Object) (Object, error) {
		return Call(d, append([]Object{self}, args...))
	})
}

// classGet applies the descriptor protocol to a class-dict value v resolved for
// the attribute name directly on the class c: a staticmethod comes back bare, a
// classmethod binds c, a property returns the descriptor itself, and a plain
// function stays unbound so an explicit-self call works.
func classGet(c *classObject, v Object) (Object, error) {
	// A descriptor subclass instance binds through its wrapped payload for
	// class-level access too, so type-level reads of abc's abstractclassmethod or
	// abstractproperty run the builtin protocol.
	if p, ok := descriptorPayload(v); ok {
		v = p
	}
	switch d := v.(type) {
	case *staticmethodObject:
		return d.fn, nil
	case *classmethodObject:
		return classmethodBind(d.fn, c), nil
	case *propertyObject:
		// Reading a property off the class hands back the descriptor itself, the
		// way CPython returns the property for type-level access.
		return d, nil
	case *instanceObject:
		// Reading a descriptor off the class runs __get__(descr, None, owner);
		// the None instance is what lets a descriptor hand back itself for
		// class-level access.
		if _, ok := d.cls.lookup("__get__"); ok {
			return instanceCallMethod(d, "__get__", []Object{None, c})
		}
	}
	return v, nil
}

// metaGet applies the descriptor protocol to a value v resolved for name on the
// metaclass meta of the class c: the class plays the part of the instance, so a
// plain function binds c as self (c.greet() calls greet(c)), a property calls
// its getter with c, a classmethod binds the metaclass, a staticmethod comes
// back bare, a user __get__ descriptor runs with c and the metaclass, and any
// other value is a plain metaclass attribute read through the class.
func metaGet(c, meta *classObject, name string, v Object) (Object, error) {
	if p, ok := descriptorPayload(v); ok {
		v = p
	}
	switch d := v.(type) {
	case *functionObject:
		return &boundMethod{fn: d, self: c}, nil
	case *staticmethodObject:
		return d.fn, nil
	case *classmethodObject:
		return classmethodBind(d.fn, meta), nil
	case *propertyObject:
		if d.fget == nil {
			return nil, Raise(AttributeError, "property '%s' of '%s' object has no getter", name, meta.name)
		}
		return Call(d.fget, []Object{c})
	case *instanceObject:
		if _, ok := d.cls.lookup("__get__"); ok {
			return instanceCallMethod(d, "__get__", []Object{c, meta})
		}
	}
	return v, nil
}

// metaSet intercepts a write to c.name when the metaclass meta defines a data
// descriptor of that name: a property runs its setter or raises the no-setter
// error, a user descriptor with __set__ runs it, and the class plays the part
// of the instance. handled is false for a plain metaclass attribute or a
// non-data descriptor, so the write lands in the class dict as before.
func metaSet(c, meta *classObject, name string, val Object) (bool, error) {
	v, ok := meta.lookup(name)
	if !ok {
		return false, nil
	}
	if p, ok := descriptorPayload(v); ok {
		v = p
	}
	switch d := v.(type) {
	case *propertyObject:
		if d.fset == nil {
			return true, Raise(AttributeError, "property '%s' of '%s' object has no setter", name, meta.name)
		}
		_, err := Call(d.fset, []Object{c, val})
		return true, err
	case *instanceObject:
		if _, ok := d.cls.lookup("__set__"); ok {
			_, err := instanceCallMethod(d, "__set__", []Object{c, val})
			return true, err
		}
	}
	return false, nil
}

// metaDel intercepts del c.name the same way metaSet intercepts a write: a
// property runs its deleter or raises the no-deleter error and a user descriptor
// with __delete__ runs it, otherwise handled is false and the class-dict entry
// is removed.
func metaDel(c, meta *classObject, name string) (bool, error) {
	v, ok := meta.lookup(name)
	if !ok {
		return false, nil
	}
	if p, ok := descriptorPayload(v); ok {
		v = p
	}
	switch d := v.(type) {
	case *propertyObject:
		if d.fdel == nil {
			return true, Raise(AttributeError, "property '%s' of '%s' object has no deleter", name, meta.name)
		}
		_, err := Call(d.fdel, []Object{c})
		return true, err
	case *instanceObject:
		if _, ok := d.cls.lookup("__delete__"); ok {
			_, err := instanceCallMethod(d, "__delete__", []Object{c})
			return true, err
		}
	}
	return false, nil
}

// classmethodBind binds cls as the first argument of a classmethod's function.
// A plain function object becomes a bound method on the class; any other
// callable is wrapped so the class is prepended at call time.
func classmethodBind(fn Object, cls *classObject) Object {
	if f, ok := fn.(*functionObject); ok {
		return &boundMethod{fn: f, self: cls}
	}
	return NewFunc("classmethod", -1, func(args []Object) (Object, error) {
		return Call(fn, append([]Object{cls}, args...))
	})
}

// propertyGetAttr reads an attribute off a property object. setter, getter, and
// deleter each return a fresh property with one slot replaced, so the
// @prop.setter chain builds up the property; fget, fset, and fdel read the
// stored callables back.
func propertyGetAttr(p *propertyObject, name string) (Object, error) {
	slotOrNone := func(o Object) Object {
		if o == nil {
			return None
		}
		return o
	}
	switch name {
	case "getter":
		return NewFunc("getter", 1, func(a []Object) (Object, error) {
			return &propertyObject{fget: a[0], fset: p.fset, fdel: p.fdel}, nil
		}), nil
	case "setter":
		return NewFunc("setter", 1, func(a []Object) (Object, error) {
			return &propertyObject{fget: p.fget, fset: a[0], fdel: p.fdel}, nil
		}), nil
	case "deleter":
		return NewFunc("deleter", 1, func(a []Object) (Object, error) {
			return &propertyObject{fget: p.fget, fset: p.fset, fdel: a[0]}, nil
		}), nil
	case "fget":
		return slotOrNone(p.fget), nil
	case "fset":
		return slotOrNone(p.fset), nil
	case "fdel":
		return slotOrNone(p.fdel), nil
	}
	return nil, Raise(AttributeError, "'property' object has no attribute '%s'", name)
}

// descriptorRepr spells the three descriptor objects deterministically, since
// their addresses are not stable.
func descriptorRepr(o Object) string {
	switch o.(type) {
	case *staticmethodObject:
		return "<staticmethod object>"
	case *classmethodObject:
		return "<classmethod object>"
	case *propertyObject:
		return "<property object>"
	case *cachedPropertyObject:
		return "<cached_property object>"
	}
	return fmt.Sprintf("<%s object>", o.TypeName())
}
