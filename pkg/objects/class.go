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
	// subclasses holds the direct subclasses that name this class among their
	// bases, appended as each is built, so __subclasses__() reports them the way
	// _py_abc.__subclasscheck__ walks them. CPython holds these weakly; the floor
	// never collects a class, so a strong list is faithful and simpler.
	subclasses []*classObject
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
	// builtinBase names the builtin type this class derives from, empty for a
	// plain object-rooted class. A dict base makes instances dict-backed: the
	// mapping protocol and dict methods route to the instance's dictData store
	// unless the class overrides them. An int base makes instances int-backed:
	// numeric, comparison, hash, conversion, and format route to the payload the
	// same way. It is inherited, so a subclass of such a subclass keeps the base.
	builtinBase string
	// builtinBaseFn is the builtin type object the base named, kept so the value
	// subclasses (int, str) can construct their payload through it, reusing the
	// builtin's own conversion. It is nil for a dict base and a plain class.
	builtinBaseFn *funcObject
	// namedBase is set when the class derives from a collections.namedtuple
	// result, the shape tokenize's TokenInfo takes. Such a class records
	// builtinBase "tuple" so its instances are tuple-backed, and namedBase carries
	// the field metadata and the builder so construction binds the fields and the
	// _make/_replace/_fields helpers resolve. It is inherited like builtinBase.
	namedBase *namedTupleType
	// annotations holds the class __annotations__ mapping, the dict its body
	// accumulated for its PEP 526 variable annotations. It lives here rather than
	// in dict so C.__annotations__ reads it while `'__annotations__' in C.__dict__`
	// stays false, the PEP 649 shape 3.14 reports. It is lazily created as an empty
	// dict on first read of a class that declared none.
	annotations Object
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
	// dictData is the mapping payload of a dict subclass instance, the storage
	// self[key] and the inherited dict methods read and write. It is nil for an
	// ordinary instance whose class has no dict base.
	dictData *dictObject
	// builtinData is the immutable payload of a value subclass instance: the
	// intObject an int subclass wraps or the strObject a str subclass wraps, set
	// once at construction. It is nil for an instance whose class has no such
	// base. The operators unwrap to it after an override lookup misses.
	builtinData Object
	// localData holds the per-thread attribute stores of a threading.local
	// subclass instance, the layout a class whose builtinBase is "local" takes.
	// It is nil for every ordinary instance; when set, the attribute protocol
	// swaps the accessing thread's private dict into attrs for the duration of a
	// read or write, so each thread sees its own instance attributes while the
	// class methods and class attributes stay shared.
	localData *localInstanceData
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
		// __annotations__ lives in a dedicated field, not the class dict, so a
		// class reads C.__annotations__ while `'__annotations__' in C.__dict__`
		// stays false the way 3.14 reports under PEP 649. A class body that
		// accumulated annotations passes them through here under this name.
		if n == "__annotations__" {
			c.annotations = vals[i]
			continue
		}
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
		if nt, ok := b.(*namedTupleType); ok {
			// A collections.namedtuple result is a tuple subclass, so a class that
			// derives from it takes the tuple layout and records the field metadata
			// in namedBase. Its instances are tuple-backed value subclasses whose
			// payload carries the fields, so the tuple operators and the namedtuple
			// field/helper reads both route to the payload.
			c.builtinBase = "tuple"
			c.namedBase = nt
			continue
		}
		if name, ok := builtinBaseName(b); ok {
			// A builtin type base like dict is not a classObject, so it never
			// joins the MRO; it is recorded as the layout the instances take. A
			// value base (int, str) keeps its type object too, so construction
			// can build the payload through the builtin's own conversion.
			c.builtinBase = name
			if fn, ok := b.(*funcObject); ok {
				c.builtinBaseFn = fn
			}
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
		if bc.builtinBase != "" {
			c.builtinBase = bc.builtinBase
			if bc.builtinBaseFn != nil {
				c.builtinBaseFn = bc.builtinBaseFn
			}
			if bc.namedBase != nil {
				c.namedBase = bc.namedBase
			}
		}
		c.bases = append(c.bases, bc)
		bc.subclasses = append(bc.subclasses, c)
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
	registerPickleClass(c)
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

// NewType3Kw builds a class from the four-argument type(name, bases, namespace,
// **kwds) form. It derives the winning metaclass from the bases the way
// type.__new__ does, so a base carrying a metaclass such as EnumType drives
// creation through that metaclass, and forwards the class keywords to it and to
// __init_subclass__. With no metaclass among the bases the winner is the default
// type metatype and the keywords reach __init_subclass__. This is the shape
// enum's convert_class runs: type(name, (StrEnum,), body, boundary=..., _simple=True).
func NewType3Kw(nameArg, basesArg, nsArg Object, kwNames []string, kwVals []Object) (Object, error) {
	if len(kwNames) == 0 {
		return typeNewCore(nil, nameArg, basesArg, nsArg)
	}
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
	winner, err := determineMeta(typeClass, bases.elts)
	if err != nil {
		return nil, err
	}
	module := "__main__"
	if m, ok, err := ns.lookup(NewStr("__module__")); err == nil && ok {
		if ms, ok := m.(*strObject); ok {
			module = ms.v
		}
	}
	return callMetaclass(winner, name.v, module+"."+name.v, bases.elts, ns, kwNames, kwVals)
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
		_, err := fn.bind(mainThread, []Object{c}, kwNames, kwVals)
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
	// Collect the (name, value) pairs first, the way type.__new__ gathers the
	// descriptor list before firing any hook. A hook like enum's _proto_member
	// deletes its own name from the class during the call, and walking the live
	// order while it shrinks would skip the entry that slid into the freed slot.
	type setNameTarget struct {
		name string
		inst *instanceObject
	}
	var targets []setNameTarget
	for _, name := range c.order {
		inst, ok := c.dict[name].(*instanceObject)
		if !ok {
			continue
		}
		if _, ok := inst.cls.lookup("__set_name__"); !ok {
			continue
		}
		targets = append(targets, setNameTarget{name: name, inst: inst})
	}
	for _, t := range targets {
		if _, err := instanceCallMethod(t.inst, "__set_name__", []Object{c, NewStr(t.name)}); err != nil {
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
	case "__dict__":
		return classDictProxy(c), true
	case "__subclasses__":
		// A bound zero-argument method reporting the direct subclasses, the live
		// list _py_abc.__subclasscheck__ iterates. It closes over the class, so a
		// class created after this read still shows up on the next call.
		return NewFunc("__subclasses__", 0, func([]Object) (Object, error) {
			return classSubclassesList(c), nil
		}), true
	}
	return nil, false
}

// classSubclassesList builds the list __subclasses__() returns, the direct
// subclasses in creation order. Each element is the class object itself, so a
// caller reads back the same classes it built.
func classSubclassesList(c *classObject) Object {
	elts := make([]Object, len(c.subclasses))
	for i, sc := range c.subclasses {
		elts[i] = sc
	}
	return NewList(elts)
}

// objectNewBuiltin is the canonical object.__new__: one shared builtin so
// object.__new__, an inherited class.__new__, and None.__new__ read back the
// same object, which is what enum's `x is object.__new__` check and the set
// membership in _find_new_ rely on. Called as object.__new__(cls) it allocates
// a bare instance of cls through the object-root allocation path, the same one
// a cooperative super().__new__ chain ends on.
var objectNewBuiltin Object

func init() {
	objectNewBuiltin = NewFunc("__new__", -1, func(args []Object) (Object, error) {
		r, ok, err := objectDefaultCall(None, "__new__", args)
		if err != nil {
			return nil, err
		}
		if !ok || r == nil {
			return nil, Raise(TypeError, "object.__new__(): not enough arguments")
		}
		return r, nil
	})
}

// objectDunders holds object's default dunder methods as canonical builtins:
// one shared object per name, stored so class-level access reads the same
// object every time and every class that does not override the name inherits
// it. That identity is what EnumType.__new__ leans on when it compares
// getattr(enum_class, name) against getattr(object, name) to decide whether a
// name is still the object default it should replace with the Enum method.
var objectDunders map[string]Object

func init() {
	objectDunders = map[string]Object{
		"__repr__": NewFunc("__repr__", 1, func(args []Object) (Object, error) {
			if len(args) != 1 {
				return nil, Raise(TypeError, "object.__repr__() takes no arguments")
			}
			return NewStr(objectDefaultRepr(args[0])), nil
		}),
		"__str__": NewFunc("__str__", 1, func(args []Object) (Object, error) {
			if len(args) != 1 {
				return nil, Raise(TypeError, "object.__str__() takes no arguments")
			}
			// object.__str__ falls back to the object-level repr.
			return NewStr(objectDefaultRepr(args[0])), nil
		}),
		"__format__": NewFunc("__format__", 2, func(args []Object) (Object, error) {
			if len(args) != 2 {
				return nil, Raise(TypeError, "object.__format__() takes exactly one argument")
			}
			spec, ok := args[1].(*strObject)
			if !ok {
				return nil, Raise(TypeError, "__format__() argument must be str, not %s", args[1].TypeName())
			}
			if spec.v != "" {
				return nil, Raise(TypeError, "unsupported format string passed to %s.__format__", args[0].TypeName())
			}
			s, err := StrE(args[0])
			if err != nil {
				return nil, err
			}
			return NewStr(s), nil
		}),
		"__init__": NewFunc("__init__", -1, func(args []Object) (Object, error) {
			// object.__init__ is a no-op that returns None. When a class defines its
			// own __new__ but no __init__, CPython lets object.__init__ swallow the
			// extra constructor arguments, which is the shape enum member creation
			// runs when it calls enum_member.__init__(*args) on a value-only member.
			return None, nil
		}),
		"__reduce_ex__": NewFunc("__reduce_ex__", 2, func(args []Object) (Object, error) {
			// Pickling support is not on the floor yet. The name has to resolve so
			// EnumType.__new__ can read and reassign it, but a real call is out of
			// scope until copyreg lands.
			return nil, Raise(TypeError, "cannot pickle '%s' object yet", func() string {
				if len(args) > 0 {
					return args[0].TypeName()
				}
				return "object"
			}())
		}),
		"__subclasshook__": NewFunc("__subclasshook__", -1, func([]Object) (Object, error) {
			// object.__subclasshook__ is the default a class inherits when it does
			// not define its own. It always returns NotImplemented so an ABCMeta
			// __subclasscheck__ can call cls.__subclasshook__(subclass) blind and
			// fall through to its registry when the class has no structural test. A
			// class that overrides the name shadows this via its own dict, which the
			// class attribute lookup consults first.
			return NotImplemented, nil
		}),
	}

	// int and str carry their own string dunders and inherit the rest from
	// object, which is the identity EnumType.__new__ reads when it borrows the
	// member type's __repr__/__str__/__format__ onto the enum class. A name that
	// resolves to the same object as object's is one the type inherited.
	builtinTypeDunders = map[string]map[string]Object{
		"int": {
			"__new__":       NewFunc("__new__", -1, builtinNewDunder),
			"__repr__":      NewFunc("__repr__", 1, builtinReprDunder),
			"__format__":    NewFunc("__format__", 2, builtinFormatDunder),
			"__str__":       objectDunders["__str__"],
			"__reduce_ex__": objectDunders["__reduce_ex__"],
		},
		"str": {
			"__new__":       NewFunc("__new__", -1, builtinNewDunder),
			"__repr__":      NewFunc("__repr__", 1, builtinReprDunder),
			"__str__":       NewFunc("__str__", 1, builtinStrDunder),
			"__format__":    NewFunc("__format__", 2, builtinFormatDunder),
			"__reduce_ex__": objectDunders["__reduce_ex__"],
		},
		// tuple carries __new__ so tuple.__new__(cls, iterable) resolves off the
		// type object and builds a value subclass instance, the allocator
		// codecs.CodecInfo(tuple) calls in its __new__. Its repr prints the
		// underlying tuple; the other dunders come from object.
		"tuple": {
			"__new__":       NewFunc("__new__", -1, builtinNewDunder),
			"__repr__":      NewFunc("__repr__", 1, builtinReprDunder),
			"__reduce_ex__": objectDunders["__reduce_ex__"],
		},
		// weakref.ref carries __hash__ so WeakMethod can borrow it with
		// `__hash__ = ref.__hash__` at class-body time, the one ref attribute
		// weakref.py reads at import. __new__ builds a ref subclass instance from
		// the referent and optional callback, the allocator a ref subclass inherits.
		"ref": {
			"__new__": NewFunc("__new__", -1, builtinNewDunder),
			"__hash__": NewFunc("__hash__", 1, func(args []Object) (Object, error) {
				h, err := PyHash(args[0])
				if err != nil {
					return nil, err
				}
				return NewInt(int64(h)), nil
			}),
		},
	}

	// The string dunders are instance-method wrappers: read off an instance they
	// bind it as self, so a class that reassigns one to another slot (Enum sets
	// __str__ = int.__repr__) still calls it with self. __new__ stays a
	// staticmethod, so it is left unbound.
	for _, name := range []string{"__repr__", "__str__", "__format__", "__reduce_ex__"} {
		markSelfBound(objectDunders[name])
	}
	for _, tbl := range builtinTypeDunders {
		for name, d := range tbl {
			if name == "__new__" {
				continue
			}
			markSelfBound(d)
		}
	}
}

// markSelfBound flags a builtin funcObject as an instance-method wrapper so a
// read off an instance binds self.
func markSelfBound(o Object) {
	if f, ok := o.(*funcObject); ok {
		f.selfBound = true
	}
}

// builtinNewDunder is int.__new__ and str.__new__ read off the type object: a
// distinct allocator from object.__new__, so int.__new__ is not object.__new__.
// Called as str.__new__(subclass, value) it builds an instance of the subclass
// carrying the payload, reusing the value-subclass allocation the cooperative
// super().__new__ chain ends on.
func builtinNewDunder(args []Object) (Object, error) {
	r, ok, err := objectDefaultCall(None, "__new__", args)
	if err != nil {
		return nil, err
	}
	if !ok || r == nil {
		return nil, Raise(TypeError, "__new__(): not enough arguments")
	}
	return r, nil
}

// builtinTypeDunders maps a builtin type name to the string dunder methods it
// resolves off the type object. An own method is a fresh builtin; an inherited
// name points at object's canonical builtin so the identity check against
// object matches CPython.
var builtinTypeDunders map[string]map[string]Object

// unwrapForDunder reads the builtin payload of a value subclass instance so a
// type dunder like int.__repr__ works on the underlying int rather than the
// subclass, and passes any other value straight through.
func unwrapForDunder(o Object) Object {
	if v, ok := builtinUnwrap(o); ok {
		return v
	}
	return o
}

// builtinReprDunder is int.__repr__ and str.__repr__: the type's own repr of the
// underlying value, bypassing any subclass override.
func builtinReprDunder(args []Object) (Object, error) {
	if len(args) != 1 {
		return nil, Raise(TypeError, "expected 1 argument, got %d", len(args))
	}
	return NewStr(Repr(unwrapForDunder(args[0]))), nil
}

// builtinStrDunder is str.__str__: the underlying string value.
func builtinStrDunder(args []Object) (Object, error) {
	if len(args) != 1 {
		return nil, Raise(TypeError, "expected 1 argument, got %d", len(args))
	}
	s, err := StrE(unwrapForDunder(args[0]))
	if err != nil {
		return nil, err
	}
	return NewStr(s), nil
}

// builtinFormatDunder is int.__format__ and str.__format__: the type's format of
// the underlying value against the given spec.
func builtinFormatDunder(args []Object) (Object, error) {
	if len(args) != 2 {
		return nil, Raise(TypeError, "__format__() takes exactly one argument")
	}
	spec, ok := args[1].(*strObject)
	if !ok {
		return nil, Raise(TypeError, "__format__() argument must be str, not %s", args[1].TypeName())
	}
	return Format(unwrapForDunder(args[0]), spec.v)
}

// objectDunderBound answers an object dunder read off an instance: the object
// default bound to the instance, so a call supplies self and inst.__repr__()
// works. The bound wrapper is a fresh builtin each read, which matches the
// instance-method case where only the class-level attribute has stable identity.
func objectDunderBound(self Object, name string) (Object, bool) {
	base, ok := objectDunders[name]
	if !ok {
		return nil, false
	}
	fn := func(args []Object) (Object, error) {
		return Call(base, append([]Object{self}, args...))
	}
	return NewFunc(name, -1, fn), true
}

// objectDefaultRepr is the object-root repr, the identity form a type that does
// not override __repr__ reports. An instance reads its qualified name and
// address; any other object falls back to its type name and address.
func objectDefaultRepr(o Object) string {
	if inst, ok := o.(*instanceObject); ok {
		return instanceRepr(inst)
	}
	return fmt.Sprintf("<%s object at %p>", o.TypeName(), o)
}

// BuiltinTypeResolver returns the type object registered under a builtin type
// name, so this package can name int or bool as an element of another builtin
// type's linearization. The runtime installs it in its init because that is
// where the builtin constructors live; a nil resolver just means only the
// object tail resolves, which is enough for the common (T, object) chain.
var BuiltinTypeResolver func(name string) (Object, bool)

// ClassOfResolver returns the type object a value reports through __class__, the
// same object type(x) yields. The runtime installs it in its init from TypeOf,
// where the builtin constructors live; LoadAttr uses it to answer __class__ for
// the scalar and container builtins, which have no dedicated case of their own,
// so _py_abc's __instancecheck__ read `instance.__class__` succeeds on 42 or a
// bare list. A nil resolver leaves those reads an AttributeError, the prior
// behavior.
var ClassOfResolver func(o Object) Object

// typeBuiltinOrClass returns the runtime `type` constructor when it is
// registered, so a default class or builtin type reports the very object the
// `type` name binds and `D.__class__ is type` holds. Before the runtime installs
// its resolver only the internal typeClass exists, which is the right stand-in.
func typeBuiltinOrClass() Object {
	if BuiltinTypeResolver != nil {
		if t, ok := BuiltinTypeResolver("type"); ok {
			return t
		}
	}
	return typeClass
}

// builtinTypeBaseNames maps a builtin type to its direct base names. Every
// builtin type in builtinTypeReprs derives straight from object except bool,
// whose base is int, so only the exceptions are listed and the rest default to
// object.
var builtinTypeBaseNames = map[string][]string{
	"bool": {"int"},
}

// builtinTypeChain is the linearization of a builtin type by name, from the type
// itself up through its bases to object: int gives [int, object] and bool gives
// [bool, int, object].
func builtinTypeChain(name string) []string {
	chain := []string{name}
	cur := name
	for {
		bases, ok := builtinTypeBaseNames[cur]
		if !ok || len(bases) == 0 {
			break
		}
		cur = bases[0]
		chain = append(chain, cur)
	}
	return append(chain, "object")
}

// builtinTypeElem resolves one name in a builtin type's chain to its type
// object: object is the root singleton, the type itself is x, and an
// intermediate base comes from the runtime resolver. A name that does not
// resolve is left out so the tuple still forms.
func builtinTypeElem(name string, x *funcObject) (Object, bool) {
	if name == "object" {
		return objectClass, true
	}
	if name == x.name {
		return x, true
	}
	if BuiltinTypeResolver != nil {
		return BuiltinTypeResolver(name)
	}
	return nil, false
}

// builtinTypeIntrospect answers the type-object attributes a builtin type
// constructor carries: __mro__ is the linearization tuple, __bases__ the direct
// bases, __base__ the primary base, and __qualname__ the type name.
func builtinTypeIntrospect(x *funcObject, name string) (Object, bool) {
	switch name {
	case "__mro__":
		var elts []Object
		for _, n := range builtinTypeChain(x.name) {
			if e, ok := builtinTypeElem(n, x); ok {
				elts = append(elts, e)
			}
		}
		return NewTuple(elts), true
	case "__bases__":
		bases := builtinTypeBaseNames[x.name]
		if len(bases) == 0 {
			bases = []string{"object"}
		}
		var elts []Object
		for _, n := range bases {
			if e, ok := builtinTypeElem(n, x); ok {
				elts = append(elts, e)
			}
		}
		return NewTuple(elts), true
	case "__base__":
		base := "object"
		if bases := builtinTypeBaseNames[x.name]; len(bases) > 0 {
			base = bases[0]
		}
		if e, ok := builtinTypeElem(base, x); ok {
			return e, true
		}
		return nil, false
	case "__qualname__":
		return NewStr(x.name), true
	case "__dict__":
		return builtinTypeDictProxy(x), true
	}
	return nil, false
}

// builtinTypeDictProxy is a builtin type's __dict__, a read-only mappingproxy
// over its own namespace. A constructor type defines __new__, so the proxy
// carries that name, which is the membership probe enum's _find_data_type_ uses
// to recognize a data type among a new enum's bases. The value under the name is
// the constructor itself, enough for the read-and-membership use the floor makes
// of it.
func builtinTypeDictProxy(x *funcObject) Object {
	d := &dictObject{index: map[string]int{}}
	_ = d.set(NewStr("__new__"), x)
	// __hash__ is present on every builtin type that defines its own, which the
	// structural Hashable check `"__hash__" in T.__dict__` then reads: a hashable
	// type carries the callable, an unhashable one (list, dict, set, bytearray)
	// carries None so the check breaks out to NotImplemented. bool is the one
	// hashable type that defines no __hash__ of its own, inheriting int's down the
	// MRO, so it is left absent for the walk to find on int.
	if x.name != "bool" {
		if builtinUnhashableType[x.name] {
			_ = d.set(NewStr("__hash__"), None)
		} else {
			_ = d.set(NewStr("__hash__"), builtinHashDunder)
		}
	}
	// The `type` metatype alone carries the __annotations__ getset descriptor;
	// annotationlib reads `type.__dict__["__annotations__"].__get__` at import.
	if x.name == "type" {
		_ = d.set(NewStr("__annotations__"), typeAnnotationsDescriptor)
	}
	return &mappingProxyObject{d: d}
}

// builtinUnhashableType names the builtin container types whose instances are
// unhashable, so their type carries __hash__ = None rather than a callable, the
// way list.__dict__["__hash__"] is None in CPython.
var builtinUnhashableType = map[string]bool{
	"list": true, "dict": true, "set": true, "bytearray": true,
}

// builtinHashDunder is the __hash__ a builtin type exposes in its __dict__: a
// callable so the structural Hashable check sees a truthy entry, backed by the
// value's own hash so type(x).__dict__["__hash__"](x) equals hash(x). It is
// assigned in init so the closure over PyHash does not form a package
// initialization cycle.
var builtinHashDunder Object

func init() {
	builtinHashDunder = NewFunc("__hash__", 1, func(args []Object) (Object, error) {
		h, err := PyHash(args[0])
		if err != nil {
			return nil, err
		}
		return NewInt(int64(h)), nil
	})
}

// classDictProxy is __dict__, the read-only mappingproxy over the class
// namespace. The entries come back in definition order, the order enum's
// classdict.update(cls.__dict__) folds them in, over a snapshot dict so a write
// through the proxy is the TypeError CPython raises for a mappingproxy.
func classDictProxy(c *classObject) Object {
	d := &dictObject{index: map[string]int{}}
	for _, name := range c.order {
		_ = d.set(NewStr(name), c.dict[name])
	}
	return &mappingProxyObject{d: d}
}

// classBases is the __bases__ tuple: the direct bases in written order, with
// the implicit object root filled in for a class that names no base. object
// itself has no bases.
func classBases(c *classObject) Object {
	if c == objectClass {
		return NewTuple(nil)
	}
	elts := make([]Object, 0, len(c.bases)+1)
	for _, b := range c.bases {
		elts = append(elts, b)
	}
	// A value subclass records its builtin base (tuple, str, int) as a layout
	// string rather than a classObject, so it is missing from the user-base
	// slice; add it as a direct base the way class T(tuple) reports (tuple,). A
	// class that only inherits the layout through a user base already lists that
	// base, so the builtin is not a direct base of it and is left off.
	if be, ok := builtinBaseElem(c); ok && !inheritsBuiltinBase(c) {
		elts = append(elts, be)
	}
	if len(elts) == 0 {
		elts = append(elts, objectClass)
	}
	return NewTuple(elts)
}

// classMroChain is the __mro__ tuple: the stored linearization with the value
// base and object root appended, since c3Linearize omits both. A value subclass
// carries its builtin base as a layout string, so it never lands in c.mro; it
// belongs just before object, the place class T(tuple).__mro__ shows tuple.
// object's own chain is just itself.
func classMroChain(c *classObject) []Object {
	if c == objectClass {
		return []Object{objectClass}
	}
	elts := make([]Object, 0, len(c.mro)+2)
	for _, k := range c.mro {
		elts = append(elts, k)
	}
	if be, ok := builtinBaseElem(c); ok {
		elts = append(elts, be)
	}
	return append(elts, objectClass)
}

// classBase is __base__, the single primary base: object for a root class,
// None for object itself, the first written base when there is one, and the
// builtin base for a bare value subclass like class T(tuple). The
// most-derived-base rule for multiple inheritance is a later slice.
func classBase(c *classObject) Object {
	if c == objectClass {
		return None
	}
	if len(c.bases) > 0 {
		return c.bases[0]
	}
	if be, ok := builtinBaseElem(c); ok {
		return be
	}
	return objectClass
}

// builtinBaseElem returns the type object a value subclass names as its builtin
// base, the element CPython places in __bases__ and __mro__ between the class
// and object. It is the constructor recorded when the base was written directly
// (class T(tuple) keeps the tuple funcObject), otherwise the type registered
// under the base name. A class with no builtin base answers a miss.
func builtinBaseElem(c *classObject) (Object, bool) {
	if c.builtinBase == "" {
		return nil, false
	}
	if c.builtinBaseFn != nil {
		return c.builtinBaseFn, true
	}
	if BuiltinTypeResolver != nil {
		if e, ok := BuiltinTypeResolver(c.builtinBase); ok {
			return e, true
		}
	}
	return nil, false
}

// inheritsBuiltinBase reports whether the class takes its builtin layout through
// a user base rather than naming the builtin directly. When a base already
// carries the same layout, the builtin is not a direct base of this class, so
// __bases__ lists the user base alone the way class U(T) shows (T,).
func inheritsBuiltinBase(c *classObject) bool {
	for _, b := range c.bases {
		if b.builtinBase == c.builtinBase {
			return true
		}
	}
	return false
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
	// A dict subclass instance is an instance of dict, so isinstance(x, dict)
	// answers from the layout the class recorded, not its own type name.
	if inst, ok := obj.(*instanceObject); ok {
		return inst.cls.builtinBase == name
	}
	switch name {
	case "type":
		return isTypeArg(obj)
	case "int":
		switch obj.(type) {
		case *intObject, *boolObject:
			return true
		}
		return false
	case "tuple":
		// namedtuple and structseq values (os.stat_result) are tuple subclasses,
		// so they report a distinct TypeName yet are instances of tuple.
		_, ok := obj.(*tupleObject)
		return ok
	case "collections.OrderedDict":
		// The OrderedDict type name carries its module so it reprs as a class the
		// way CPython does, but an OrderedDict value reprs bare as OrderedDict({...})
		// and so reports the bare TypeName. Bridge the two here.
		return obj.TypeName() == "OrderedDict"
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
	r, err := fn.bind(mainThread, []Object{c, arg}, nil, nil)
	if err != nil {
		return nil, true, err
	}
	t, err := TruthOf(r)
	if err != nil {
		return nil, true, err
	}
	return NewBool(t), true, nil
}

// classMetaIter dispatches iteration over a class object to its metaclass
// __iter__, the hook a metaclass like EnumType defines so `for m in Cls` and
// tuple unpacking walk the class rather than its instances. It reports
// handled=false when the class rides the default type metatype or its metaclass
// defines no __iter__, so Iter keeps the plain "type object is not iterable"
// error. The result of __iter__ is run back through Iter, matching iter() which
// takes the iterator the hook hands back.
func classMetaIter(c *classObject) (Iterator, bool, error) {
	meta, ok := userMetaclass(c)
	if !ok {
		return nil, false, nil
	}
	v, ok := meta.lookup("__iter__")
	if !ok {
		return nil, false, nil
	}
	fn, ok := v.(*functionObject)
	if !ok {
		return nil, false, nil
	}
	r, err := fn.bind(mainThread, []Object{c}, nil, nil)
	if err != nil {
		return nil, true, err
	}
	it, err := Iter(r)
	return it, true, err
}

// classMetaLen runs a metaclass __len__ for len(cls), the way EnumType.__len__
// counts a Color enum's members. handled is false when the metaclass carries no
// __len__, so the caller falls through to the not-sized TypeError.
func classMetaLen(c *classObject) (Object, bool, error) {
	meta, ok := userMetaclass(c)
	if !ok {
		return nil, false, nil
	}
	v, ok := meta.lookup("__len__")
	if !ok {
		return nil, false, nil
	}
	fn, ok := v.(*functionObject)
	if !ok {
		return nil, false, nil
	}
	r, err := fn.bind(mainThread, []Object{c}, nil, nil)
	return r, true, err
}

// classMetaContains runs a metaclass __contains__ for item in cls, the way
// EnumType.__contains__ decides Color.RED in Color. handled is false when the
// metaclass carries no __contains__, so the caller falls back to iteration.
func classMetaContains(c *classObject, item Object) (Object, bool, error) {
	meta, ok := userMetaclass(c)
	if !ok {
		return nil, false, nil
	}
	v, ok := meta.lookup("__contains__")
	if !ok {
		return nil, false, nil
	}
	fn, ok := v.(*functionObject)
	if !ok {
		return nil, false, nil
	}
	res, err := fn.bind(mainThread, []Object{c, item}, nil, nil)
	if err != nil {
		return nil, true, err
	}
	// CPython runs the __contains__ result through PyObject_IsTrue, so a returned
	// object with a __bool__ decides membership.
	t, err := TruthOf(res)
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
	if u, ok := cls.(*unionObject); ok {
		// isinstance against a union checks membership in any of its types, the
		// same way a tuple of types does.
		for _, e := range u.args {
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
	// A user class is a subclass of a builtin type only when it derives from
	// that type, the layout recorded on the class; object is handled above.
	if tname, ok := builtinTypeArgName(cls); ok {
		return NewBool(sc.builtinBase == tname), nil
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
	return InstantiateT(mainThread, c, pos, kwNames, kwVals)
}

// InstantiateT builds an instance for the constructing thread t. It matters only
// for a threading.local subclass, whose __init__ runs on the thread that creates
// the instance; every other class ignores t and builds the same instance a t-less
// call would. CallT threads the real caller here so `L(...)` on a worker thread
// primes that worker's per-thread dict.
func InstantiateT(t *Thread, c *classObject, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	// A threading.local subclass takes the per-thread attribute layout: its
	// instance carries one attribute dict per thread and re-runs __init__ on each
	// thread's first access, so it never reaches the ordinary instance path.
	if c.builtinBase == "local" && !c.isMeta {
		return instantiateLocal(t, c, pos, kwNames, kwVals)
	}
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
				return fn.bind(mainThread, append([]Object{Object(c)}, pos...), kwNames, kwVals)
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
		if err := initSelf(init, e, pos, kwNames, kwVals); err != nil {
			return nil, err
		}
		return e, nil
	}
	inst := &instanceObject{cls: c, attrs: newAttrs()}
	if c.namedBase != nil {
		// A namedtuple subclass with no __new__ of its own builds its tuple payload
		// through the namedtuple builder, which binds the fields positionally or by
		// keyword and fills the defaults, so C(field=value) and C(*row) both work.
		// The keywords are the fields, so an inherited __init__ ignores them.
		v, err := CallKw(c.namedBase.build, pos, kwNames, kwVals)
		if err != nil {
			return nil, err
		}
		inst.builtinData = v
		if init, ok := c.lookup("__init__"); ok {
			if err := initSelf(init, inst, nil, nil, nil); err != nil {
				return nil, err
			}
		}
		return inst, nil
	}
	switch c.builtinBase {
	case "dict":
		inst.dictData = &dictObject{index: map[string]int{}}
	case "int", "str", "tuple", "classmethod", "staticmethod", "property", "ref":
		// A value subclass builds its immutable payload through the builtin base's
		// own conversion, the way int.__new__ or str.__new__ sets the value from
		// the constructor arguments before __init__ runs. classmethod, staticmethod
		// and property build their wrapped-descriptor payload the same way, from the
		// constructor arguments. A ref subclass wraps the weakref the base ref(...)
		// call builds from the referent and optional callback. The keyword arguments
		// belong to a user __init__, so only the positional ones reach the value.
		v, err := Call(c.builtinBaseFn, pos)
		if err != nil {
			return nil, err
		}
		inst.builtinData = v
	case "types.GenericAlias":
		// A GenericAlias subclass built directly, X(origin, item), wraps the
		// parameterized generic as its payload. It carries no builtinBaseFn, so
		// the value is built through NewGenericAlias rather than a base call. The
		// two positional arguments are origin and the argument tuple, matching
		// types.GenericAlias(origin, args).
		v, err := newGenericAliasPayload(pos)
		if err != nil {
			return nil, err
		}
		inst.builtinData = v
	}
	init, ok := c.lookup("__init__")
	if !ok {
		if inst.dictData != nil {
			// A dict subclass with no __init__ override inherits dict.__init__,
			// which seeds the store from the constructor arguments.
			if err := dictInit(inst.dictData, pos, kwNames, kwVals); err != nil {
				return nil, err
			}
			return inst, nil
		}
		if inst.builtinData != nil {
			// A value subclass with no __init__ override: the payload was set from
			// the arguments and the inherited __init__ ignores them.
			return inst, nil
		}
		if len(pos) > 0 || len(kwNames) > 0 {
			return nil, Raise(TypeError, "%s() takes no arguments", c.name)
		}
		return inst, nil
	}
	if err := initSelf(init, inst, pos, kwNames, kwVals); err != nil {
		return nil, err
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
	return initSelf(init, obj, pos, kwNames, kwVals)
}

// initSelf runs a resolved __init__ against self with the constructor arguments
// and enforces CPython's None-return rule. A plain def-statement function binds
// directly; any other callable, such as the builtin NewMethod __init__ a
// Go-built classObject carries, dispatches through CallKw with self prepended.
func initSelf(init, self Object, pos []Object, kwNames []string, kwVals []Object) error {
	return initSelfT(mainThread, init, self, pos, kwNames, kwVals)
}

// initSelfT runs __init__ under a given thread, the threaded core initSelf wraps
// with the main thread. A threading.local subclass re-runs __init__ on each
// thread that first touches the instance, so the store spine inside that
// __init__ must carry the accessing thread rather than the main one.
func initSelfT(t *Thread, init, self Object, pos []Object, kwNames []string, kwVals []Object) error {
	withSelf := append([]Object{self}, pos...)
	var ret Object
	var err error
	if fn, ok := init.(*functionObject); ok {
		ret, err = fn.bind(t, withSelf, kwNames, kwVals)
	} else {
		ret, err = CallKwT(t, init, withSelf, kwNames, kwVals)
	}
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
	// __class__ answers type(x) for every value that gives the name no meaning of
	// its own. An instance reads its stored class, a class its metaclass, and a
	// builtin function its metatype, all in their own cases below; the scalar and
	// container builtins have no such case, so they route through the resolver
	// here. This is what lets _py_abc's __instancecheck__ read `instance.__class__`
	// on a plain int or list.
	if name == "__class__" && ClassOfResolver != nil {
		switch o.(type) {
		case *instanceObject, *classObject, *funcObject:
			// dedicated __class__ semantics in their own case
		default:
			return ClassOfResolver(o), nil
		}
	}
	switch x := o.(type) {
	case *instanceObject:
		return instanceLoadAttr(x, name)
	case *intObject, *boolObject:
		return intLoadAttr(o, name)
	case *floatObject:
		return floatLoadAttr(o, name)
	case *unionObject:
		return unionLoadAttr(x, name)
	case *Module:
		return moduleLoadAttr(x, name)
	case *classObject:
		// __name__, __qualname__, __bases__, __mro__, and __base__ are
		// metaclass data descriptors, so they answer from the type object
		// itself and outrank anything the class body bound under those names.
		if v, ok := classIntrospect(x, name); ok {
			return v, nil
		}
		// __class__ is a type-level data descriptor too, so a class answers its
		// metaclass: a class built through a metaclass reads that metaclass, a
		// default class reads the `type` builtin so `D.__class__ is type` holds.
		// Enum's _create_ reads it as metacls = cls.__class__ for the functional
		// Enum('Name', names) API.
		if name == "__class__" {
			if meta, ok := userMetaclass(x); ok {
				return meta, nil
			}
			return typeBuiltinOrClass(), nil
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
		// A namedtuple subclass answers the class-level namedtuple helpers off its
		// recorded field metadata, so TokenInfo._fields and TokenInfo._make resolve
		// the way they do on the namedtuple base, with _make building a subclass
		// instance rather than a bare namedtuple.
		if x.namedBase != nil {
			if v, ok := namedClassAttr(x, name); ok {
				return v, nil
			}
		}
		// A class with no __new__ of its own inherits object.__new__, so
		// C.__new__ reads back the one canonical allocator.
		if name == "__new__" {
			return objectNewBuiltin, nil
		}
		// A class that overrides none of the object dunders inherits them, so
		// C.__repr__ is object.__repr__ the way CPython reports it.
		if v, ok := objectDunders[name]; ok {
			return v, nil
		}
		// Every class carries __doc__: a class with no docstring reads it back as
		// None rather than raising, the way CPython's type object does. Vendored
		// io.py leans on this when it copies `_io._IOBase.__doc__` onto its own
		// abstract bases.
		if name == "__doc__" {
			return None, nil
		}
		// Every class carries __annotations__: the dict its body accumulated, or a
		// fresh empty one for a class that declared none, the way CPython's type
		// object answers rather than raising. A class that did accumulate one bound
		// it in the class dict, so x.lookup above already returned it.
		if name == "__annotations__" {
			return classAnnotations(x), nil
		}
		return nil, Raise(AttributeError, "type object '%s' has no attribute '%s'", x.name, name)
	case *annotationsDescriptor:
		// annotationlib binds `type.__dict__["__annotations__"].__get__` once and
		// calls it to read a class's annotations, so the descriptor answers that
		// one attribute with the reader and nothing else.
		if name == "__get__" {
			return NewFunc("__get__", -1, annotationsDescriptorGet), nil
		}
		return nil, Raise(AttributeError, "'getset_descriptor' object has no attribute '%s'", name)
	case *staticmethodObject:
		if name == "__func__" || name == "__wrapped__" {
			return x.fn, nil
		}
		return nil, Raise(AttributeError, "'staticmethod' object has no attribute '%s'", name)
	case *classmethodObject:
		if name == "__func__" || name == "__wrapped__" {
			return x.fn, nil
		}
		return nil, Raise(AttributeError, "'classmethod' object has no attribute '%s'", name)
	case *propertyObject:
		return propertyGetAttr(x, name)
	case *cachedPropertyObject:
		return cachedPropertyAttr(x, name)
	case *genericAliasObject:
		return genericAliasLoadAttr(x, name)
	case *typeAliasObject:
		return typeAliasLoadAttr(x, name)
	case *templateObject:
		return templateLoadAttr(x, name)
	case *interpolationObject:
		return interpolationLoadAttr(x, name)
	case *frameObject:
		return frameLoadAttr(x, name)
	case *codeObject:
		return codeLoadAttr(x, name)
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
	case *contextVar:
		if name == "name" {
			return NewStr(x.name), nil
		}
		return nil, Raise(AttributeError, "'ContextVar' object has no attribute '%s'", name)
	case *contextToken:
		switch name {
		case "var":
			return x.variable, nil
		case "old_value":
			if x.hadOld {
				return x.oldValue, nil
			}
			return tokenMissing, nil
		}
		return nil, Raise(AttributeError, "'Token' object has no attribute '%s'", name)
	case *tokenClass:
		if name == "MISSING" {
			return tokenMissing, nil
		}
		return nil, Raise(AttributeError, "type object 'Token' has no attribute '%s'", name)
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
		// A dict method read binds as a callable, so d.get reads back and calls
		// the same as d.get(...). Counter and OrderedDict each add their own names
		// on top of the shared dict surface.
		if dictMethodNames[name] ||
			(x.kind == counterDict && counterExtraMethodNames[name]) ||
			(x.kind == orderedDict && orderedExtraMethodNames[name]) {
			return builtinMethodValue(x, name), nil
		}
		if v, ok := containerSpecialAttr(x, name); ok {
			return v, nil
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
	case *chainMapObject:
		// maps is the live list of underlying mappings, the one mutable data
		// attribute; parents is a computed property, a ChainMap of every map but
		// the first. Any method name binds as a callable so cm.get reads back the
		// same as cm.get(...); everything else is the plain attribute miss.
		switch name {
		case "maps":
			return x.maps, nil
		case "parents":
			return chainMapParents(x)
		}
		if chainMapMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
		return nil, noAttr(x, name)
	case *arrayObject:
		return arrayLoadAttr(x, name)
	case *csvReader:
		return csvReaderLoadAttr(x, name)
	case *csvWriter:
		return csvWriterLoadAttr(x, name)
	case *csvDialect:
		return csvDialectAttr(x, name)
	case *hashObject:
		return hashLoadAttr(x, name)
	case *hmacObject:
		return hmacLoadAttr(x, name)
	case *listObject:
		// A list method read binds as a callable, so items.append reads back and
		// calls the same as items.append(x); any other name is the plain miss.
		if listMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
		if v, ok := containerSpecialAttr(x, name); ok {
			return v, nil
		}
		return nil, noAttr(x, name)
	case *memoryviewObject:
		return memoryviewLoadAttr(x, name)
	case *threadObject:
		return threadLoadAttr(x, name)
	case *lockObject:
		if lockMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
		return nil, noAttr(x, name)
	case *rlockObject:
		if rlockMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
		return nil, noAttr(x, name)
	case *condObject:
		if condMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
		return nil, noAttr(x, name)
	case *eventObject:
		if eventMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
		return nil, noAttr(x, name)
	case *semaphoreObject:
		if semaphoreMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
		return nil, noAttr(x, name)
	case *barrierObject:
		if barrierProperties[name] {
			return barrierProperty(x, name), nil
		}
		if barrierMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
		return nil, noAttr(x, name)
	case *queueObject:
		if queueProperties[name] {
			return queueProperty(x, name), nil
		}
		if queueMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
		return nil, noAttr(x, name)
	case *simpleQueueObject:
		if simpleQueueMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
		return nil, noAttr(x, name)
	case *futureObject:
		if futureMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
		return nil, noAttr(x, name)
	case *asyncFuture:
		if asyncFutureMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
		return nil, noAttr(x, name)
	case *asyncioBarrier:
		switch name {
		case "parties":
			return NewInt(int64(x.parties)), nil
		case "n_waiting":
			return NewInt(int64(x.nWaiting())), nil
		case "broken":
			return NewBool(x.broken()), nil
		}
		if asyncioBarrierMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
		return nil, noAttr(x, name)
	case *executorObject:
		if executorMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
		return nil, noAttr(x, name)
	case *eventLoop:
		if eventLoopMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
		return nil, noAttr(x, name)
	case *asyncioServer:
		if name == "sockets" {
			return asyncioServerSockets(x), nil
		}
		if asyncioServerMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
		return nil, noAttr(x, name)
	case *asyncioSocket:
		if name == "getsockname" {
			return builtinMethodValue(x, name), nil
		}
		return nil, noAttr(x, name)
	case *localObject:
		// A threading.local read reaches here only when it arrives through the
		// thread-agnostic LoadAttr, which the t-less spine routes with the main
		// thread; the emitted x.attr code carries the real thread via LoadAttrT.
		return x.loadAttr(mainThread, name)
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
		// A type object reports its metatype through __class__: int.__class__ is
		// type, while a plain builtin function is builtin_function_or_method.
		if name == "__class__" {
			if builtinTypeReprs[x.name] {
				return typeBuiltinOrClass(), nil
			}
			return TypeSingleton("builtin_function_or_method"), nil
		}
		// A constructor that doubles as a type object answers its string dunder
		// methods and the type introspection attributes: int.__format__,
		// int.__mro__, str.__bases__, bool.__base__.
		if builtinTypeReprs[x.name] {
			if v, ok := builtinTypeClassmethod(x.name, name); ok {
				return v, nil
			}
			if tbl, ok := builtinTypeDunders[x.name]; ok {
				if v, ok := tbl[name]; ok {
					return v, nil
				}
			}
			if v, ok := builtinTypeIntrospect(x, name); ok {
				return v, nil
			}
		}
	case *typeObject:
		if name == "__name__" || name == "__qualname__" {
			return NewStr(x.name), nil
		}
		// A constructor-less builtin type still answers the type introspection
		// attributes, so _collections_abc can register coroutine, generator, and
		// the iterator and view types: `_check_methods` opens with `mro = C.__mro__`
		// and probes `method in B.__dict__` for each B in it. The floor gives every
		// such type the plain (T, object) chain rooted at object; its own methods
		// are not enumerated in __dict__ yet, so a structural subclass check returns
		// NotImplemented and the ABC falls back to its registry, which register just
		// populated, keeping isinstance faithful.
		switch name {
		case "__mro__":
			return NewTuple([]Object{x, objectClass}), nil
		case "__bases__":
			return NewTuple([]Object{objectClass}), nil
		case "__base__":
			return objectClass, nil
		case "__dict__":
			d := &dictObject{index: map[string]int{}}
			return &mappingProxyObject{d: d}, nil
		}
		return nil, Raise(AttributeError, "type object '%s' has no attribute '%s'", x.name, name)
	case *namedTupleType:
		return namedTupleTypeAttr(x, name)
	case *tupleGetterObject:
		return tupleGetterAttr(x, name)
	case *StructSeqType:
		return structSeqTypeAttr(x, name)
	case *partialObject:
		return partialAttr(x, name)
	case *patternObject:
		return patternAttr(x, name)
	case *matchObject:
		return matchAttr(x, name)
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
		if x.sseq != nil {
			return structSeqInstanceAttr(x, name)
		}
		// A plain tuple reads back its count and index methods as callables; the
		// membership and subscript protocol still falls through to the shared
		// container handling below.
		if tupleMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
	case *strObject:
		// A method read binds as a callable, "ab".upper reads back and calls the
		// same as "ab".upper(); anything else drops to the shared container
		// handling below, so "ab".__len__ still resolves.
		if strMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
	case *bytesObject:
		if bytesMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
	case *bytearrayObject:
		if bytearrayMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
	case *setObject:
		if setMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
	case *frozensetObject:
		if frozensetMethodNames[name] {
			return builtinMethodValue(x, name), nil
		}
	case *noneObject:
		// NoneType inherits object.__new__, so None.__new__ resolves to the
		// same allocator every other type reports, which is what enum's
		// _find_new_ set literal relies on when it names None.__new__.
		if name == "__new__" {
			return objectNewBuiltin, nil
		}
	}
	// A builtin container with no dedicated case above (str, bytes, tuple,
	// range, set, frozenset) exposes its size, membership and subscript
	// protocol methods as bound reads, so frozenset(kwlist).__contains__ hands
	// back a callable rather than raising.
	if v, ok := containerSpecialAttr(o, name); ok {
		return v, nil
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
	case *threadObject:
		return threadStoreAttr(x, name, val)
	case *tupleGetterObject:
		return tupleGetterSet(x, name, val)
	case *localObject:
		// See LoadAttr: a t-less write lands on the main thread's private store;
		// the emitted code carries the real thread via StoreAttrT.
		x.storeAttr(mainThread, name, val)
		return nil
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
	case *chainMapObject:
		// cm.maps is reassignable: some code swaps the whole list of mappings.
		// The attribute is a plain public list in CPython, so store whatever object
		// is handed over and let the later maps operations read it back.
		if name == "maps" {
			x.maps = val
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
	case *localObject:
		// See LoadAttr: a t-less delete targets the main thread's private store;
		// the emitted code carries the real thread via DelAttrT.
		return x.delAttr(mainThread, name)
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
	// cls[key] resolves through the type's type first, so a metaclass
	// __getitem__ runs with the class as self before the __class_getitem__
	// fallback; enum's EnumType.__getitem__ is the member-by-name lookup that
	// makes Color['GREEN'] work.
	if meta, ok := userMetaclass(c); ok {
		if v, ok := meta.lookup("__getitem__"); ok {
			if fn, ok := v.(*functionObject); ok {
				return Call(fn, []Object{c, item})
			}
		}
	}
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
	return instanceCallMethodT(mainThread, x, name, args)
}

// instanceCallMethodT is instanceCallMethod threading the ambient Thread into
// the resolved method, so a method called on an instance inside a child thread
// runs under that thread and its identity lookups are honest.
func instanceCallMethodT(t *Thread, x *instanceObject, name string, args []Object) (Object, error) {
	v, err := LoadAttr(x, name)
	if err != nil {
		return nil, err
	}
	return CallT(t, v, args)
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

// classCallMethodT dispatches Cls.name(args): the name resolves on the class
// through the descriptor protocol (a plain function stays unbound so self is
// explicit, a classmethod binds the class, a staticmethod is bare), then the
// callable takes the arguments under the ambient Thread.
func classCallMethodT(t *Thread, x *classObject, name string, args []Object) (Object, error) {
	v, err := LoadAttr(x, name)
	if err != nil {
		return nil, err
	}
	return CallT(t, v, args)
}

// instanceCallMethodKwT is instance.name(...) with keyword arguments: the
// attribute resolves through the descriptor protocol, then the keywords reach
// the resolved callable's binder under the ambient Thread.
func instanceCallMethodKwT(t *Thread, x *instanceObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	v, err := LoadAttr(x, name)
	if err != nil {
		return nil, err
	}
	return CallKwT(t, v, pos, kwNames, kwVals)
}

// classCallMethodKwT is Cls.name(...) with keyword arguments, so the class
// attribute resolves through the descriptor protocol and the keywords land on
// the resolved callable's own binder under the ambient Thread.
func classCallMethodKwT(t *Thread, x *classObject, name string, pos []Object, kwNames []string, kwVals []Object) (Object, error) {
	v, err := LoadAttr(x, name)
	if err != nil {
		return nil, err
	}
	return CallKwT(t, v, pos, kwNames, kwVals)
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
