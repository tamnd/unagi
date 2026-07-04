package objects

import (
	"fmt"
	"sort"
	"strings"
)

// objectClass is the implicit root type every class linearizes to and every
// value is an instance of. The lowering models the object base as a nil entry
// and c3Linearize leaves it off the stored mro, so this singleton is
// synthesized only where the class surface must name it: the __bases__,
// __mro__, and __base__ reads and isinstance and issubclass over the root.
var objectClass = &classObject{name: "object", qual: "object", dict: map[string]Object{}}

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
	// order records the insertion order of the instance's own attributes, so
	// __dict__ and vars() report them the way CPython's ordered instance dict
	// does. It holds every live key in dict; StoreAttr appends a new key and
	// DelAttr removes it.
	order []string
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
func NewClass(name, qual string, bases []Object, names []string, vals []Object, kwNames []string, kwVals []Object) (Object, error) {
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
	if err := c.runSetNameHooks(); err != nil {
		return nil, err
	}
	if err := c.runInitSubclass(kwNames, kwVals); err != nil {
		return nil, err
	}
	return c, nil
}

// runInitSubclass fires the nearest base's __init_subclass__ on the new class,
// after __set_name__, the order type.__new__ uses. __init_subclass__ is an
// implicit classmethod, so the new class is passed as the leading argument and
// the class keyword arguments follow. The lookup skips the class itself and
// scans its ancestors; a class defines the hook for its subclasses, not for
// itself. When only object's default is left, keyword arguments are the
// no-keyword-arguments TypeError and an empty call is a no-op.
//
// A user hook that chains with super().__init_subclass__() needs the
// classmethod super(type, subtype) form, which is a later slice; a self
// contained hook that consumes its keywords works today.
func (c *classObject) runInitSubclass(kwNames []string, kwVals []Object) error {
	for _, base := range c.mro[1:] {
		v, ok := base.dict["__init_subclass__"]
		if !ok {
			continue
		}
		fn, ok := v.(*functionObject)
		if !ok {
			return Raise(TypeError, "__init_subclass__ must be a plain function in this tier")
		}
		_, err := fn.bind([]Object{c}, kwNames, kwVals)
		return err
	}
	if len(kwNames) > 0 {
		return Raise(TypeError, "%s.__init_subclass__() takes no keyword arguments", c.name)
	}
	return nil
}

// runSetNameHooks fires __set_name__(owner, name) on every value the class body
// bound whose type defines it, in definition order, right after the class
// object exists. This is the descriptor-registration hook CPython runs from
// type.__new__: a Field-style descriptor learns the attribute name it was
// assigned to. Only names set in this body take part, never inherited ones, so
// it walks c.order. A raising hook propagates; the RuntimeError that CPython
// wraps it in is a later refinement.
func (c *classObject) runSetNameHooks() error {
	for _, name := range c.order {
		inst, ok := c.dict[name].(*instanceObject)
		if !ok {
			continue
		}
		if _, ok := inst.cls.lookup("__set_name__"); !ok {
			continue
		}
		if _, err := instanceCallMethod(inst, "__set_name__", []Object{c, NewStr(name)}); err != nil {
			return err
		}
	}
	return nil
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

// classIntrospect answers the type object's own read-only attributes. Each
// value is computed from the class rather than stored, and the implicit object
// root is materialized here so __bases__ and __mro__ spell it the way CPython
// does. A name outside this set returns false so LoadAttr falls through to the
// MRO lookup.
func classIntrospect(c *classObject, name string) (Object, bool) {
	switch name {
	case "__name__":
		return NewStr(c.name), true
	case "__qualname__":
		// The stored qual carries the module prefix repr wants; __qualname__ is
		// that path without the module, which is the bare name for a top-level
		// class and Outer.Inner for a nested one.
		return NewStr(strings.TrimPrefix(c.qual, "__main__.")), true
	case "__bases__":
		return classBases(c), true
	case "__mro__":
		return NewTuple(classMroChain(c)), true
	case "__base__":
		return classBase(c), true
	}
	return nil, false
}

// classBases is the __bases__ tuple: the direct bases in written order, with
// the implicit object root filled in for a class that names no base. object
// itself has no bases.
func classBases(c *classObject) Object {
	if c == objectClass {
		return NewTuple(nil)
	}
	if len(c.bases) == 0 {
		return NewTuple([]Object{objectClass})
	}
	elts := make([]Object, len(c.bases))
	for i, b := range c.bases {
		elts[i] = b
	}
	return NewTuple(elts)
}

// classMroChain is the __mro__ tuple: the stored linearization with the object
// root appended, since c3Linearize omits it. object's own chain is just itself.
func classMroChain(c *classObject) []Object {
	if c == objectClass {
		return []Object{objectClass}
	}
	elts := make([]Object, 0, len(c.mro)+1)
	for _, k := range c.mro {
		elts = append(elts, k)
	}
	return append(elts, objectClass)
}

// classBase is __base__, the single primary base: object for a root class,
// None for object itself, and the first written base otherwise. The
// most-derived-base rule for multiple inheritance is a later slice.
func classBase(c *classObject) Object {
	if c == objectClass {
		return None
	}
	if len(c.bases) == 0 {
		return objectClass
	}
	return c.bases[0]
}

// builtinTypeArgName reports the type name carried by a value used as a type
// argument to isinstance, issubclass, or a class pattern: a builtin constructor
// like int or the metatype type, or a constructor-less type singleton like
// type(None). It does not accept user classes (those take the classObject path)
// or exception matchers.
func builtinTypeArgName(cls Object) (string, bool) {
	switch c := cls.(type) {
	case *funcObject:
		if builtinTypeReprs[c.name] {
			return c.name, true
		}
	case *typeObject:
		return c.name, true
	}
	return "", false
}

// isTypeArg reports whether o is itself a type value, the test isinstance(o,
// type) runs. It extends IsTypeValue to the builtin constructors, which are
// funcObjects rather than typeObjects yet still reprs as classes.
func isTypeArg(o Object) bool {
	if IsTypeValue(o) {
		return true
	}
	if f, ok := o.(*funcObject); ok {
		return builtinTypeReprs[f.name]
	}
	return false
}

// instanceOfBuiltin reports whether obj is an instance of the builtin type
// named name. int owns bool as a subtype, type covers every type value, and
// every other kind matches its own TypeName exactly.
func instanceOfBuiltin(obj Object, name string) bool {
	switch name {
	case "type":
		return isTypeArg(obj)
	case "int":
		switch obj.(type) {
		case *intObject, *boolObject:
			return true
		}
		return false
	default:
		return obj.TypeName() == name
	}
}

// IsInstance implements isinstance(obj, cls). cls is a class, a builtin type, or
// a tuple of those; anything else raises the arg 2 TypeError probed on 3.14. A
// user instance matches when cls is in its MRO or is the object root; a builtin
// value matches when its kind is or descends from the named builtin type.
func IsInstance(obj, cls Object) (Object, error) {
	if t, ok := cls.(*tupleObject); ok {
		for _, e := range t.elts {
			r, err := IsInstance(obj, e)
			if err != nil {
				return nil, err
			}
			if r == True {
				return True, nil
			}
		}
		return False, nil
	}
	if c, ok := cls.(*classObject); ok {
		if c == objectClass {
			return True, nil
		}
		inst, ok := obj.(*instanceObject)
		if !ok {
			return False, nil
		}
		for _, k := range inst.cls.mro {
			if k == c {
				return True, nil
			}
		}
		return False, nil
	}
	if name, ok := builtinTypeArgName(cls); ok {
		if instanceOfBuiltin(obj, name) {
			return True, nil
		}
		return False, nil
	}
	return nil, Raise(TypeError, "isinstance() arg 2 must be a type, a tuple of types, or a union")
}

// IsSubclass implements issubclass(sub, cls). sub is validated as a class or a
// builtin type first, matching CPython's arg 1 check that fires before arg 2;
// cls is a class, a builtin type, or a tuple of those. Every class is a
// subclass of object.
func IsSubclass(sub, cls Object) (Object, error) {
	if sc, ok := sub.(*classObject); ok {
		return subclassOf(sc, cls)
	}
	if sname, ok := builtinTypeArgName(sub); ok {
		return builtinSubclassOf(sname, cls)
	}
	return nil, Raise(TypeError, "issubclass() arg 1 must be a class")
}

func subclassOf(sc *classObject, cls Object) (Object, error) {
	if t, ok := cls.(*tupleObject); ok {
		for _, e := range t.elts {
			r, err := subclassOf(sc, e)
			if err != nil {
				return nil, err
			}
			if r == True {
				return True, nil
			}
		}
		return False, nil
	}
	if c, ok := cls.(*classObject); ok {
		if c == objectClass {
			return True, nil
		}
		if sc == objectClass {
			return False, nil
		}
		for _, k := range sc.mro {
			if k == c {
				return True, nil
			}
		}
		return False, nil
	}
	// A user class is a subclass of no builtin type other than object.
	if _, ok := builtinTypeArgName(cls); ok {
		return False, nil
	}
	return nil, Raise(TypeError, "issubclass() arg 2 must be a class, a tuple of classes, or a union")
}

// builtinSubclassOf answers issubclass when the first argument is a builtin
// type. A builtin type descends only from itself, from object, and bool from
// int; it is never a subclass of a user class.
func builtinSubclassOf(sname string, cls Object) (Object, error) {
	if t, ok := cls.(*tupleObject); ok {
		for _, e := range t.elts {
			r, err := builtinSubclassOf(sname, e)
			if err != nil {
				return nil, err
			}
			if r == True {
				return True, nil
			}
		}
		return False, nil
	}
	if c, ok := cls.(*classObject); ok {
		if c == objectClass {
			return True, nil
		}
		return False, nil
	}
	if tname, ok := builtinTypeArgName(cls); ok {
		if sname == tname || (tname == "int" && sname == "bool") {
			return True, nil
		}
		return False, nil
	}
	return nil, Raise(TypeError, "issubclass() arg 2 must be a class, a tuple of classes, or a union")
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

// delAttr removes a class attribute and drops it from the insertion order, so
// a later re-add appends fresh rather than reusing the old slot.
func (c *classObject) delAttr(name string) {
	delete(c.dict, name)
	for i, n := range c.order {
		if n == name {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
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
		// __dict__ is a data descriptor on the type, so it answers from the
		// instance itself and outranks anything in the instance dict.
		if name == "__dict__" {
			return instanceDict(x)
		}
		// CPython precedence: a data descriptor on the type outranks the
		// instance dict, then the instance dict, then a non-data descriptor or
		// plain class value, then AttributeError.
		tv, tok := x.cls.lookup(name)
		if tok && isDataDescriptor(tv) {
			return instanceGet(x, name, tv)
		}
		if v, ok := x.dict[name]; ok {
			return v, nil
		}
		if tok {
			return instanceGet(x, name, tv)
		}
		// A class __getattr__ is the last resort: normal resolution missed, so it
		// is called with the name and its own AttributeError propagates. The
		// lookup here cannot re-enter this miss path, since __getattr__ is found
		// on the class as a plain method.
		if _, ok := x.cls.lookup("__getattr__"); ok {
			return instanceCallMethod(x, "__getattr__", []Object{NewStr(name)})
		}
		return nil, Raise(AttributeError, "'%s' object has no attribute '%s'", x.cls.name, name)
	case *classObject:
		// __name__, __qualname__, __bases__, __mro__, and __base__ are
		// metaclass data descriptors, so they answer from the type object
		// itself and outrank anything the class body bound under those names.
		if v, ok := classIntrospect(x, name); ok {
			return v, nil
		}
		if v, ok := x.lookup(name); ok {
			return classGet(x, v)
		}
		return nil, Raise(AttributeError, "type object '%s' has no attribute '%s'", x.name, name)
	case *propertyObject:
		return propertyGetAttr(x, name)
	case *superObject:
		return superLoadAttr(x, name)
	case *Exception:
		return excLoadAttr(x, name)
	case *complexObject:
		switch name {
		case "real":
			return NewFloat(x.re), nil
		case "imag":
			return NewFloat(x.im), nil
		}
		return nil, Raise(AttributeError, "'complex' object has no attribute '%s'", name)
	case *funcObject:
		// Builtin functions and the constructor-backed type objects carry a
		// __name__/__qualname__, so type(5).__name__ and len.__name__ read back.
		if name == "__name__" || name == "__qualname__" {
			return NewStr(x.name), nil
		}
	case *typeObject:
		if name == "__name__" || name == "__qualname__" {
			return NewStr(x.name), nil
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
		// A data descriptor on the type intercepts the write: a property calls
		// its setter, or raises the probed no-setter error when it has none, and
		// a user descriptor with __set__ runs __set__(descr, instance, value).
		if tv, ok := x.cls.lookup(name); ok {
			switch d := tv.(type) {
			case *propertyObject:
				if d.fset == nil {
					return Raise(AttributeError, "property '%s' of '%s' object has no setter", name, x.cls.name)
				}
				_, err := Call(d.fset, []Object{x, val})
				return err
			case *instanceObject:
				if _, ok := d.cls.lookup("__set__"); ok {
					_, err := instanceCallMethod(d, "__set__", []Object{x, val})
					return err
				}
			}
		}
		if _, seen := x.dict[name]; !seen {
			x.order = append(x.order, name)
		}
		x.dict[name] = val
		return nil
	case *classObject:
		x.setAttr(name, val)
		return nil
	case *Exception:
		if handled, err := excStoreAttr(x, name, val); handled {
			return err
		}
	}
	return Raise(AttributeError,
		"'%s' object has no attribute '%s' and no __dict__ for setting new attributes",
		o.TypeName(), name)
}

// DelAttr implements del o.name. On an instance a data descriptor with
// __delete__ intercepts the delete, a property runs its deleter or raises the
// no-deleter error, and otherwise the instance-dict entry is removed, missing
// name being the same AttributeError a read gives. On a class the class-dict
// entry is removed, missing name spelling the type-object wording.
func DelAttr(o Object, name string) error {
	switch x := o.(type) {
	case *instanceObject:
		if tv, ok := x.cls.lookup(name); ok {
			switch d := tv.(type) {
			case *propertyObject:
				if d.fdel == nil {
					return Raise(AttributeError, "property '%s' of '%s' object has no deleter", name, x.cls.name)
				}
				_, err := Call(d.fdel, []Object{x})
				return err
			case *instanceObject:
				if _, ok := d.cls.lookup("__delete__"); ok {
					_, err := instanceCallMethod(d, "__delete__", []Object{x})
					return err
				}
			}
		}
		if _, ok := x.dict[name]; !ok {
			return Raise(AttributeError, "'%s' object has no attribute '%s'", x.cls.name, name)
		}
		delete(x.dict, name)
		for i, k := range x.order {
			if k == name {
				x.order = append(x.order[:i], x.order[i+1:]...)
				break
			}
		}
		return nil
	case *classObject:
		if _, ok := x.dict[name]; !ok {
			return Raise(AttributeError, "type object '%s' has no attribute '%s'", x.name, name)
		}
		x.delAttr(name)
		return nil
	}
	return Raise(AttributeError, "'%s' object has no attribute '%s'", o.TypeName(), name)
}

// instanceDict builds a fresh dict of an instance's own attributes in insertion
// order, backing `inst.__dict__` and `vars(inst)`. CPython hands back a live
// mapping; this is a snapshot, so writing through the returned dict does not
// reach the instance. Keys missing from the order slice (only possible for
// whitebox-constructed instances that bypass StoreAttr) are appended in sorted
// order so the result stays deterministic.
func instanceDict(x *instanceObject) (Object, error) {
	keys := make([]Object, 0, len(x.dict))
	vals := make([]Object, 0, len(x.dict))
	seen := make(map[string]bool, len(x.dict))
	for _, k := range x.order {
		v, ok := x.dict[k]
		if !ok || seen[k] {
			continue
		}
		seen[k] = true
		keys = append(keys, NewStr(k))
		vals = append(vals, v)
	}
	if len(seen) < len(x.dict) {
		rest := make([]string, 0, len(x.dict)-len(seen))
		for k := range x.dict {
			if !seen[k] {
				rest = append(rest, k)
			}
		}
		sort.Strings(rest)
		for _, k := range rest {
			keys = append(keys, NewStr(k))
			vals = append(vals, x.dict[k])
		}
	}
	return NewDict(keys, vals)
}

// InstanceDict exposes an instance's own attributes as an ordered dict for the
// vars() builtin. A non-instance has no __dict__, which is the TypeError vars()
// gives on 3.14.
func InstanceDict(o Object) (Object, error) {
	x, ok := o.(*instanceObject)
	if !ok {
		return nil, Raise(TypeError, "vars() argument must have __dict__ attribute")
	}
	return instanceDict(x)
}

// classSubscript implements C[item] on a class. Subscription looks for a
// __class_getitem__ on the class, which CPython treats as an implicit
// classmethod, so cls is prepended whether it was written plain or with an
// explicit @classmethod. A class without the hook is not subscriptable.
func classSubscript(c *classObject, item Object) (Object, error) {
	v, ok := c.lookup("__class_getitem__")
	if !ok {
		return nil, Raise(TypeError, "type '%s' is not subscriptable", c.name)
	}
	if cm, ok := v.(*classmethodObject); ok {
		return Call(classmethodBind(cm.fn, c), []Object{item})
	}
	return Call(v, []Object{c, item})
}

// instanceCallMethod dispatches inst.name(args). It resolves the attribute
// through the same descriptor protocol LoadAttr uses, so a plain method binds
// self, a staticmethod is called bare, a classmethod binds the type, and a
// property's value is called; then the resolved callable takes the arguments.
func instanceCallMethod(x *instanceObject, name string, args []Object) (Object, error) {
	v, err := LoadAttr(x, name)
	if err != nil {
		return nil, err
	}
	return Call(v, args)
}

// instanceSpecial invokes a special method looked up on the instance's type,
// the implicit-invocation path CPython uses for dunders like __repr__: it
// skips the instance dict, binds self through the descriptor protocol, and
// calls the result. defined is false when the type holds no such method, so
// the caller falls back to the default behavior for the operation.
func instanceSpecial(x *instanceObject, name string, args ...Object) (res Object, defined bool, err error) {
	tv, ok := x.cls.lookup(name)
	if !ok {
		return nil, false, nil
	}
	bound, err := instanceGet(x, name, tv)
	if err != nil {
		return nil, true, err
	}
	res, err = Call(bound, args)
	return res, true, err
}

// instanceLookupBound resolves a special method on the instance's type and binds
// self through the descriptor protocol without calling it, so a caller that has
// keyword arguments can forward them to the bound callable's own binder. defined
// is false when the type holds no such method.
func instanceLookupBound(x *instanceObject, name string) (bound Object, defined bool, err error) {
	tv, ok := x.cls.lookup(name)
	if !ok {
		return nil, false, nil
	}
	bound, err = instanceGet(x, name, tv)
	if err != nil {
		return nil, true, err
	}
	return bound, true, nil
}

// classCallMethod dispatches Cls.name(args): the name resolves on the class
// through the descriptor protocol (a plain function stays unbound so self is
// explicit, a classmethod binds the class, a staticmethod is bare), then the
// callable takes the arguments.
func classCallMethod(x *classObject, name string, args []Object) (Object, error) {
	v, err := LoadAttr(x, name)
	if err != nil {
		return nil, err
	}
	return Call(v, args)
}

// instanceCallMethodKw is instanceCallMethod with keyword arguments: the
// attribute resolves through the descriptor protocol, then the keywords reach
// the resolved callable's binder.
func instanceCallMethodKw(x *instanceObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	v, err := LoadAttr(x, name)
	if err != nil {
		return nil, err
	}
	return CallKw(v, pos, kwNames, kwVals)
}

// classCallMethodKw is classCallMethod with keyword arguments, so the class
// attribute resolves through the descriptor protocol and the keywords land on
// the resolved callable's own binder.
func classCallMethodKw(x *classObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	v, err := LoadAttr(x, name)
	if err != nil {
		return nil, err
	}
	return CallKw(v, pos, kwNames, kwVals)
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
