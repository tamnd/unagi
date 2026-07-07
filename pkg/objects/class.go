package objects

import (
	"fmt"
	"strings"
)

// objectClass is the implicit root type every class linearizes to and every
// value is an instance of. The lowering models the object base as a nil entry
// and c3Linearize leaves it off the stored mro, so this singleton is
// synthesized only where the class surface must name it: the __bases__,
// __mro__, and __base__ reads and isinstance and issubclass over the root.
var objectClass = &classObject{name: "object", qual: "object", dict: map[string]Object{}}

// ObjectType is the object builtin, the root type every class derives from. It
// is the same singleton the MRO, __bases__, and isinstance/issubclass roots
// already resolve to, so the runtime can register it as the `object` name and
// object() constructs a bare instance through the ordinary class-call path.
func ObjectType() Object { return objectClass }

// typeClass is the `type` metatype modelled as a class, so a user metaclass can
// name it as a base and super() inside a metaclass method can reach its
// __new__ and __init__. Its mro starts with itself the way every class's does;
// the object root is appended where the surface needs it. The two slots hold
// the type-object construction and no-op initialization a metaclass inherits.
var typeClass = &classObject{name: "type", qual: "type", dict: map[string]Object{}, isMeta: true}

func init() {
	typeClass.mro = []*classObject{typeClass}
	typeClass.dict["__new__"] = NewFunc("__new__", -1, typeNew)
	typeClass.dict["__init__"] = NewFunc("__init__", -1, func([]Object) (Object, error) { return None, nil })
}

// metaOf reports a class's metaclass, defaulting to the `type` metatype for a
// class that was not built through a user metaclass.
func metaOf(c *classObject) *classObject {
	if c.meta != nil {
		return c.meta
	}
	return typeClass
}

// userMetaclass returns a class's metaclass when it is a user metaclass, the
// case where attribute reads on the class consult the metaclass MRO. A class on
// the default `type` metatype returns false so its reads stay untouched.
func userMetaclass(c *classObject) (*classObject, bool) {
	if c.meta != nil && c.meta != typeClass {
		return c.meta, true
	}
	return nil, false
}

// UserMetaOf returns the user metaclass of a class value so type() can report
// it. ok is false for a class on the default `type` metatype, which the runtime
// spells with the `type` builtin, and for every non-class object.
func UserMetaOf(o Object) (Object, bool) {
	c, ok := o.(*classObject)
	if !ok {
		return nil, false
	}
	if c.meta != nil && c.meta != typeClass {
		return c.meta, true
	}
	return nil, false
}

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
	// meta is the class's metaclass, the type object type() reports for the
	// class. A nil meta means the default `type` metatype; a user class
	// created through a metaclass carries that metaclass here. isMeta marks a
	// class that derives from type, whose instances are themselves classes.
	meta   *classObject
	isMeta bool
	// __slots__ layout, filled by applySlots at creation. slotNames are this
	// class's own mangled slot names; hasSlots marks a body that bound
	// __slots__ at all; slotsDict and slotsWeakref record the pseudo-slots.
	// instDict and instWeakref say whether instances carry a __dict__ and
	// weakref support, derived from the whole base chain: a class without
	// __slots__ anywhere keeps both.
	slotNames    []string
	hasSlots     bool
	slotsDict    bool
	slotsWeakref bool
	instDict     bool
	instWeakref  bool
}

func (*classObject) TypeName() string { return "type" }

// instanceObject is an instance of a user class. attrs is its __dict__, the
// per-instance attribute store; a name missing here falls back to the class.
// The store is a real dictObject, so obj.__dict__ hands back the live mapping
// CPython does: writing through it or aliasing it reaches the instance, and it
// keeps insertion order and identity across reads for free.
type instanceObject struct {
	cls   *classObject
	attrs *dictObject
	// slots holds the values behind the class's member descriptors, separate
	// from attrs so a mixed dict-plus-slots instance's __dict__ never shows a
	// slot value. Lazily allocated on the first slot write.
	slots map[string]Object
}

func (o *instanceObject) TypeName() string { return o.cls.name }

// newAttrs builds an empty per-instance attribute store.
func newAttrs() *dictObject { return &dictObject{index: map[string]int{}} }

// attrGet reads an own attribute by name; ok is false when the instance has no
// such key, so the caller falls back to the class.
func (o *instanceObject) attrGet(name string) (Object, bool) {
	v, ok, _ := o.attrs.lookup(NewStr(name))
	return v, ok
}

// attrSet stores an own attribute, recording insertion order through the dict.
func (o *instanceObject) attrSet(name string, val Object) {
	_ = o.attrs.set(NewStr(name), val)
}

// attrDel removes an own attribute and reports whether it was present.
func (o *instanceObject) attrDel(name string) bool {
	_, ok, _ := o.attrs.delete(NewStr(name))
	return ok
}

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
	return newClassCore(nil, name, qual, bases, names, vals, kwNames, kwVals)
}

// newClassCore builds a class object on a given metaclass, the shared body of
// the class statement, type(name, bases, ns), and a metaclass's default
// __new__. meta is the metaclass to record; a nil meta leaves the class on the
// default `type` metatype. The `type` metatype is accepted as a base so a user
// metaclass can derive from it, which marks the new class isMeta; a base that is
// itself a metaclass propagates the mark the same way.
func newClassCore(meta *classObject, name, qual string, bases []Object, names []string, vals []Object, kwNames []string, kwVals []Object) (Object, error) {
	c := &classObject{name: name, qual: qual, dict: make(map[string]Object, len(names)), meta: meta}
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
		bc, ok := asBaseClass(b)
		if !ok {
			return nil, Raise(TypeError, "bases must be types")
		}
		if seen[bc] {
			return nil, Raise(TypeError, "duplicate base class %s", bc.name)
		}
		seen[bc] = true
		if bc.isMeta {
			c.isMeta = true
		}
		c.bases = append(c.bases, bc)
	}
	// __slots__ processing installs the member descriptors and sets the layout
	// flags; the layout check then rejects bases whose slot layouts cannot
	// combine, and the instance flags fold the base chain: a dict (and weakref
	// support) survives unless every contributing class traded it away.
	if err := applySlots(c); err != nil {
		return nil, err
	}
	if err := checkLayout(c); err != nil {
		return nil, err
	}
	c.instDict = !c.hasSlots || c.slotsDict || anyBase(c, func(b *classObject) bool { return b.instDict })
	c.instWeakref = !c.hasSlots || c.slotsWeakref || anyBase(c, func(b *classObject) bool { return b.instWeakref })
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

// asBaseClass resolves a base expression to the class object it names. A user
// class is itself; the `type` builtin, however it was spelled, resolves to the
// type metatype so `class Meta(type)` derives a metaclass.
func asBaseClass(b Object) (*classObject, bool) {
	if c, ok := b.(*classObject); ok {
		return c, true
	}
	if name, ok := BuiltinFuncName(b); ok && name == "type" {
		return typeClass, true
	}
	return nil, false
}

// NewType3 builds a class from the three-argument type(name, bases, namespace)
// form: the dynamic-class path type.__new__ runs. It validates the argument
// types with the probed type.__new__ wording, unpacks the namespace dict into
// the ordered name/value pairs a class body would produce, and hands the rest
// to NewClass so C3 linearization, __set_name__ and __init_subclass__ fire the
// same way a class statement would. A namespace __module__ sets the qualified
// name so repr reads <class 'module.Name'>, defaulting to __main__ like CPython.
func NewType3(nameArg, basesArg, nsArg Object) (Object, error) {
	return typeNewCore(nil, nameArg, basesArg, nsArg)
}

// typeNew is type.__new__, the metatype constructor a metaclass inherits and
// reaches through super().__new__(mcs, name, bases, ns). The leading argument
// is the metaclass the resulting class is created on; the rest are the same
// (name, bases, namespace) triple type(name, bases, ns) takes.
func typeNew(args []Object) (Object, error) {
	if len(args) != 4 {
		return nil, Raise(TypeError, "type.__new__() takes exactly 4 arguments")
	}
	meta, ok := args[0].(*classObject)
	if !ok {
		return nil, Raise(TypeError, "type.__new__(X): X is not a type object (%s)", args[0].TypeName())
	}
	return typeNewCore(meta, args[1], args[2], args[3])
}

// typeNewCore builds a class from the (name, bases, namespace) triple on the
// given metaclass. It validates the argument types with the probed type.__new__
// wording, unpacks the namespace dict into the ordered name/value pairs a class
// body would produce, and hands the rest to newClassCore so C3 linearization,
// __set_name__ and __init_subclass__ fire the way a class statement would. A
// namespace __module__ sets the qualified name so repr reads <class
// 'module.Name'>, defaulting to __main__ like CPython.
func typeNewCore(meta *classObject, nameArg, basesArg, nsArg Object) (Object, error) {
	name, ok := nameArg.(*strObject)
	if !ok {
		return nil, Raise(TypeError, "type.__new__() argument 1 must be str, not %s", nameArg.TypeName())
	}
	bases, ok := basesArg.(*tupleObject)
	if !ok {
		return nil, Raise(TypeError, "type.__new__() argument 2 must be tuple, not %s", basesArg.TypeName())
	}
	ns, ok := nsArg.(*dictObject)
	if !ok {
		return nil, Raise(TypeError, "type.__new__() argument 3 must be dict, not %s", nsArg.TypeName())
	}
	module := "__main__"
	var names []string
	var vals []Object
	for _, e := range ns.entries {
		key, ok := e.key.(*strObject)
		if !ok {
			// CPython only warns and keeps the entry under a non-str key; this
			// tier cannot slot a non-str name and drops it, a documented edge.
			continue
		}
		if key.v == "__module__" {
			if m, ok := e.val.(*strObject); ok {
				module = m.v
			}
		}
		names = append(names, key.v)
		vals = append(vals, e.val)
	}
	return newClassCore(meta, name.v, module+"."+name.v, bases.elts, names, vals, nil, nil)
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
	// object participates in the merge as the shared root so an explicit base
	// order that puts it before another base is caught as the conflict CPython
	// reports; it is stripped from the stored mro afterward since it holds no
	// user names and is re-synthesized where the surface needs it.
	var seqs [][]*classObject
	for _, b := range c.bases {
		seqs = append(seqs, fullMRO(b))
	}
	baseSeq := append([]*classObject(nil), c.bases...)
	if !containsClass(c.bases, objectClass) {
		baseSeq = append(baseSeq, objectClass)
	}
	seqs = append(seqs, baseSeq)
	merged, err := c3Merge(seqs)
	if err != nil {
		return nil, err
	}
	full := append([]*classObject{c}, merged...)
	return dropClass(full, objectClass), nil
}

// fullMRO is a base's linearization with object restored at the tail, the form
// C3 merges over. object's own chain is just itself.
func fullMRO(b *classObject) []*classObject {
	if b == objectClass {
		return []*classObject{objectClass}
	}
	return append(append([]*classObject(nil), b.mro...), objectClass)
}

func containsClass(cs []*classObject, want *classObject) bool {
	for _, c := range cs {
		if c == want {
			return true
		}
	}
	return false
}

func dropClass(cs []*classObject, drop *classObject) []*classObject {
	out := cs[:0:0]
	for _, c := range cs {
		if c != drop {
			out = append(out, c)
		}
	}
	return out
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
		// class and Outer.Inner for a nested one. The module comes from the
		// class dict, where the class statement and type.__new__ both put it.
		mod := "__main__"
		if m, ok := c.dict["__module__"].(*strObject); ok {
			mod = m.v
		}
		return NewStr(strings.TrimPrefix(c.qual, mod+".")), true
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

// metaCheckHook consults a user metaclass override of name (__instancecheck__ for
// isinstance, __subclasscheck__ for issubclass) on the class c, calling it with c
// as self and arg as the checked object. A user override decides the answer
// outright, even against a real subclass, the way CPython routes isinstance and
// issubclass through the metatype. handled is false when c rides the default type
// metatype or its metaclass does not define name, so the caller keeps the
// structural MRO walk that reproduces type's own check.
func metaCheckHook(c *classObject, name string, arg Object) (Object, bool, error) {
	meta, ok := userMetaclass(c)
	if !ok {
		return nil, false, nil
	}
	v, ok := meta.lookup(name)
	if !ok {
		return nil, false, nil
	}
	fn, ok := v.(*functionObject)
	if !ok {
		return nil, false, nil
	}
	r, err := fn.bind([]Object{c, arg}, nil, nil)
	if err != nil {
		return nil, true, err
	}
	t, err := TruthOf(r)
	if err != nil {
		return nil, true, err
	}
	return NewBool(t), true, nil
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
		if r, handled, err := metaCheckHook(c, "__instancecheck__", obj); handled {
			return r, err
		}
		if c == objectClass {
			return True, nil
		}
		if inst, ok := obj.(*instanceObject); ok {
			for _, k := range inst.cls.mro {
				if k == c {
					return True, nil
				}
			}
			return False, nil
		}
		// A raised exception is an instance of its built-in exception class and
		// every class that class derives from, so isinstance(e, ValueError) walks
		// the exception class MRO the same way an instance's does.
		if exc, ok := obj.(*Exception); ok {
			if ec, ok := excClassOf(exc); ok {
				if ec == c {
					return True, nil
				}
				for _, k := range ec.mro {
					if k == c {
						return True, nil
					}
				}
			}
			return False, nil
		}
		// A class is an instance of its metaclass and of every metaclass that
		// metaclass derives from, so isinstance(C, Meta) walks the metaclass MRO.
		if oc, ok := obj.(*classObject); ok {
			for _, k := range metaOf(oc).mro {
				if k == c {
					return True, nil
				}
			}
			return False, nil
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
		return builtinSubclassOf(sub, sname, cls)
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
		if r, handled, err := metaCheckHook(c, "__subclasscheck__", sc); handled {
			return r, err
		}
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
func builtinSubclassOf(sub Object, sname string, cls Object) (Object, error) {
	if t, ok := cls.(*tupleObject); ok {
		for _, e := range t.elts {
			r, err := builtinSubclassOf(sub, sname, e)
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
		if r, handled, err := metaCheckHook(c, "__subclasscheck__", sub); handled {
			return r, err
		}
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
	// Calling a metaclass builds a class, not an ordinary instance: it runs the
	// metaclass __new__/__init__ protocol, so Meta(name, bases, ns) creates the
	// class the same three-argument type(...) call does.
	if c.isMeta {
		return callMetaInstance(c, pos, kwNames, kwVals)
	}
	// A user metaclass __call__ intercepts C(...): CPython invokes it as
	// type(C).__call__(C, *args, **kw), so a metaclass that caches instances or
	// tags them after creation runs instead of the default protocol. Its body
	// reaches the ordinary creation through super().__call__, which lands on
	// instantiateCore. A class on the default type metatype has no such override
	// and creates its instance directly.
	if meta, ok := userMetaclass(c); ok {
		if call, ok := meta.lookup("__call__"); ok {
			if fn, ok := call.(*functionObject); ok {
				return fn.bind(append([]Object{Object(c)}, pos...), kwNames, kwVals)
			}
		}
	}
	return instantiateCore(c, pos, kwNames, kwVals)
}

// instantiateCore is the default type.__call__ creation protocol: it runs a user
// __new__ then __init__, builds an *Exception for an exception class, or
// allocates a plain instance and initializes it. Instantiate calls it when no
// metaclass __call__ intervenes, and a metaclass __call__ reaches it through
// super().__call__.
func instantiateCore(c *classObject, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	// A user-defined __new__ runs CPython's type.__call__ creation protocol: the
	// class calls __new__(cls, *args) to allocate, and __init__ runs only when
	// the result is an instance of the class, so a __new__ that returns another
	// type skips initialization. __new__ is an implicit staticmethod, so cls is
	// passed explicitly with no self binding.
	if newRaw, ok := c.lookup("__new__"); ok {
		obj, err := CallKw(staticNew(newRaw), append([]Object{Object(c)}, pos...), kwNames, kwVals)
		if err != nil {
			return nil, err
		}
		isInst, err := IsInstance(obj, c)
		if err != nil {
			return nil, err
		}
		if isInst != True {
			return obj, nil
		}
		if err := callInit(c, obj, pos, kwNames, kwVals); err != nil {
			return nil, err
		}
		return obj, nil
	}
	// An exception class builds an *Exception, not a plain instance, so the
	// result is raisable and carries the traceback machinery. The positional
	// arguments seed args the way BaseException.__new__ does; a user __init__
	// then runs against the exception itself as self, so it can reset args
	// through super().__init__ and store attributes on the exception's dict.
	if isExcClass(c) {
		e := &Exception{Kind: c.name, Class: c, Args: pos}
		init, ok := c.lookup("__init__")
		if !ok {
			if len(kwNames) > 0 {
				return nil, Raise(TypeError, "%s() takes no keyword arguments", c.name)
			}
			return e, nil
		}
		fn, ok := init.(*functionObject)
		if !ok {
			// A non-function __init__ needs the descriptor protocol to call,
			// which waits on a later slice.
			return nil, Raise(TypeError, "%s() argument after __init__ is not a plain function", c.name)
		}
		withSelf := append([]Object{e}, pos...)
		ret, err := fn.bind(withSelf, kwNames, kwVals)
		if err != nil {
			return nil, err
		}
		if _, isNone := ret.(*noneObject); !isNone {
			return nil, Raise(TypeError, "__init__() should return None, not '%s'", ret.TypeName())
		}
		return e, nil
	}
	inst := &instanceObject{cls: c, attrs: newAttrs()}
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

// staticNew unwraps a __new__ value to the callable to invoke directly. __new__
// is an implicit staticmethod, so a plain function is called as is and an
// explicit @staticmethod wrapper unwraps to the function it holds.
func staticNew(v Object) Object {
	if sm, ok := v.(*staticmethodObject); ok {
		return sm.fn
	}
	return v
}

// callInit runs __init__ on an object a user __new__ allocated. A missing user
// __init__ leaves the object as __new__ built it: once __new__ is overridden the
// object root ignores the constructor arguments, so no arity check fires here.
// The instance is bound as self and a non-None return is the probed TypeError.
func callInit(c *classObject, obj Object, pos []Object, kwNames []string, kwVals []Object) error {
	init, ok := c.lookup("__init__")
	if !ok {
		return nil
	}
	fn, ok := init.(*functionObject)
	if !ok {
		return Raise(TypeError, "%s() argument after __init__ is not a plain function", c.name)
	}
	ret, err := fn.bind(append([]Object{obj}, pos...), kwNames, kwVals)
	if err != nil {
		return err
	}
	if _, isNone := ret.(*noneObject); !isNone {
		return Raise(TypeError, "__init__() should return None, not '%s'", ret.TypeName())
	}
	return nil
}

// LoadAttr reads o.name as a value. On an instance the instance dict wins,
// then a class function binds to the instance and a class variable comes
// back as is; on a class the name comes straight from the class dict. The
// two AttributeError wordings, instance and type object, are probed on 3.14.
func LoadAttr(o Object, name string) (Object, error) {
	switch x := o.(type) {
	case *instanceObject:
		return instanceLoadAttr(x, name)
	case *Module:
		return moduleLoadAttr(x, name)
	case *classObject:
		// __name__, __qualname__, __bases__, __mro__, and __base__ are
		// metaclass data descriptors, so they answer from the type object
		// itself and outrank anything the class body bound under those names.
		if v, ok := classIntrospect(x, name); ok {
			return v, nil
		}
		// A user metaclass contributes attributes to its classes: a data
		// descriptor on the metaclass MRO wins even over the class's own dict,
		// then the class's own MRO answers, then a plain metaclass attribute
		// binds the class as self. Default-metatype classes skip this entirely
		// so an ordinary class reads exactly as before.
		if meta, ok := userMetaclass(x); ok {
			if mv, found := meta.lookup(name); found {
				if isDataDescriptor(mv) {
					return metaGet(x, meta, name, mv)
				}
				if v, ok := x.lookup(name); ok {
					return classGet(x, v)
				}
				return metaGet(x, meta, name, mv)
			}
		}
		if v, ok := x.lookup(name); ok {
			return classGet(x, v)
		}
		return nil, Raise(AttributeError, "type object '%s' has no attribute '%s'", x.name, name)
	case *propertyObject:
		return propertyGetAttr(x, name)
	case *cachedPropertyObject:
		return cachedPropertyAttr(x, name)
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
	case *sliceObject:
		// The three parts are read-only attributes carrying whatever was
		// stored, None included.
		switch name {
		case "start":
			return x.start, nil
		case "stop":
			return x.stop, nil
		case "step":
			return x.step, nil
		}
		return nil, Raise(AttributeError, "'slice' object has no attribute '%s'", name)
	case *dictObject:
		// default_factory is the one data attribute a defaultdict exposes; a
		// plain dict has no attribute of that name. It reads the stored factory
		// or None.
		if x.kind == defaultDict && name == "default_factory" {
			return dictDefaultFactory(x), nil
		}
		return nil, noAttr(x, name)
	case *dequeObject:
		// maxlen reads the bound as an int, or None for an unbounded deque; it is
		// the one data attribute CPython's deque exposes.
		if name == "maxlen" {
			if x.bounded() {
				return NewInt(int64(x.maxlen)), nil
			}
			return None, nil
		}
		return nil, noAttr(x, name)
	case *memoryviewObject:
		return memoryviewLoadAttr(x, name)
	case *stringIOObject:
		if name == "closed" {
			return NewBool(x.closed), nil
		}
		return nil, noAttr(x, name)
	case *bytesIOObject:
		if name == "closed" {
			return NewBool(x.closed), nil
		}
		return nil, noAttr(x, name)
	case *functionObject:
		// A Python function carries the __name__/__qualname__/__doc__/__module__/
		// __annotations__ slots plus a __dict__ of arbitrary attributes; the read
		// protocol lives with the writable overlay in funcattrs.go.
		return functionLoadAttr(x, name)
	case *boundMethod:
		// A bound method exposes the function and the instance it is bound to as
		// __func__/__self__, and proxies every other attribute to the underlying
		// function, so __name__/__qualname__ read through and a miss reports the
		// function's own AttributeError, matching CPython's method getattr.
		switch name {
		case "__func__":
			return x.fn, nil
		case "__self__":
			return x.self, nil
		}
		return LoadAttr(x.fn, name)
	case *funcObject:
		// Builtin functions and the constructor-backed type objects carry a
		// __name__/__qualname__, so type(5).__name__ and len.__name__ read back.
		if name == "__name__" || name == "__qualname__" {
			return NewStr(x.name), nil
		}
		// A builtin may attach its own attributes, such as chain.from_iterable.
		if v, ok := x.attrs[name]; ok {
			return v, nil
		}
	case *typeObject:
		if name == "__name__" || name == "__qualname__" {
			return NewStr(x.name), nil
		}
		return nil, Raise(AttributeError, "type object '%s' has no attribute '%s'", x.name, name)
	case *namedTupleType:
		return namedTupleTypeAttr(x, name)
	case *partialObject:
		return partialAttr(x, name)
	case *lruCacheObject:
		return lruAttr(x, name)
	case *keyObject:
		if name == "obj" {
			if x.obj == nil {
				return None, nil
			}
			return x.obj, nil
		}
		return nil, Raise(AttributeError, "'functools.KeyWrapper' object has no attribute '%s'", name)
	case *tupleObject:
		if x.named != nil {
			return namedTupleInstanceAttr(x, name)
		}
	}
	return nil, Raise(AttributeError, "'%s' object has no attribute '%s'", o.TypeName(), name)
}

// StoreAttr writes o.name = val. Instances and classes take new attributes;
// a builtin has no __dict__, which is the wording 3.14 gives.
func StoreAttr(o Object, name string, val Object) error {
	switch x := o.(type) {
	case *instanceObject:
		return instanceStoreAttr(x, name, val)
	case *functionObject:
		return functionStoreAttr(x, name, val)
	case *Module:
		return moduleStoreAttr(x, name, val)
	case *classObject:
		// A data descriptor on a user metaclass intercepts the write the way it
		// does on an instance; a plain metaclass attribute lets the value land in
		// the class dict.
		if meta, ok := userMetaclass(x); ok {
			if handled, err := metaSet(x, meta, name, val); handled {
				return err
			}
		}
		x.setAttr(name, val)
		return nil
	case *Exception:
		if handled, err := excStoreAttr(x, name, val); handled {
			return err
		}
	case *dictObject:
		// A defaultdict's default_factory is writable: assigning None or a
		// callable changes what a later missing key produces. Any other value is
		// the "must be callable or None" TypeError CPython raises.
		if x.kind == defaultDict && name == "default_factory" {
			if val != None && !Callable(val) {
				return Raise(TypeError, "default_factory must be callable or None")
			}
			x.factory = val
			return nil
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
		return instanceDelAttr(x, name)
	case *functionObject:
		return functionDelAttr(x, name)
	case *Module:
		return moduleDelAttr(x, name)
	case *classObject:
		if meta, ok := userMetaclass(x); ok {
			if handled, err := metaDel(x, meta, name); handled {
				return err
			}
		}
		if _, ok := x.dict[name]; !ok {
			return Raise(AttributeError, "type object '%s' has no attribute '%s'", x.name, name)
		}
		x.delAttr(name)
		return nil
	}
	return Raise(AttributeError, "'%s' object has no attribute '%s'", o.TypeName(), name)
}

// instanceDict returns the live dict backing an instance's own attributes, which
// is `inst.__dict__` and `vars(inst)`. It is the actual store, not a snapshot, so
// writing through it or aliasing it reaches the instance and it keeps insertion
// order and identity across reads, matching CPython's instance dict.
func instanceDict(x *instanceObject) (Object, error) {
	return x.attrs, nil
}

// InstanceDict exposes an instance's own attributes as an ordered dict for the
// vars() builtin. A non-instance has no __dict__, which is the TypeError vars()
// gives on 3.14.
func InstanceDict(o Object) (Object, error) {
	switch x := o.(type) {
	case *instanceObject:
		return instanceDict(x)
	case *Exception:
		return excDict(x)
	}
	return nil, Raise(TypeError, "vars() argument must have __dict__ attribute")
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
