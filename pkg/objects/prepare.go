package objects

import "strings"

// A class statement builds its class through a ClassBuilder, following the
// order CPython's __build_class__ uses: determine the winning metaclass, ask
// its __prepare__ for the namespace mapping, write the compiler-synthesized
// members and then every body binding into that mapping through the item
// protocol, and finally hand the populated namespace to the metaclass. A
// custom mapping returned by __prepare__ therefore observes every write in
// definition order, and the same mapping object reaches the metaclass's
// __new__ and __init__.

// ClassBuilder carries one class statement's build from the header through
// the body to the metaclass call. meta is the winning metaclass when the
// explicit metaclass= argument is a class; callable is set instead when it is
// any other callable, which skips determination against the bases and simply
// receives (name, bases, ns) plus the class keywords, its return value bound
// to the class name whatever it is.
type ClassBuilder struct {
	meta     *classObject
	callable Object
	name     string
	qual     string
	bases    []Object
	ns       Object
	kwNames  []string
	kwVals   []Object
}

// StartClass runs the class-statement header: metaclass determination,
// __prepare__, and the synthesized namespace members CPython writes before
// the body executes, __module__, __qualname__, __firstlineno__, and __doc__
// when the body opens with a docstring (doc is nil otherwise). meta is the
// explicit metaclass= argument or nil; module is the defining module's
// __name__, which qual carries as its leading segment; kwNames and kwVals
// are the remaining class keywords, which __prepare__ receives too.
func StartClass(meta Object, module, name, qual string, firstLine int, doc Object, bases []Object, kwNames []string, kwVals []Object) (*ClassBuilder, error) {
	// An explicit metaclass that is a class, type-derived or plain, joins the
	// most-derived determination against the bases' metaclasses; anything else
	// is taken as-is, the isinstance(meta, type) split __build_class__ makes.
	var explicit *classObject
	var callable Object
	if meta != nil {
		if c, ok := asBaseClass(meta); ok {
			explicit = c
		} else {
			callable = meta
		}
	}
	var winner *classObject
	var ns Object
	var err error
	if callable != nil {
		ns, err = prepareCallable(callable, name, bases, kwNames, kwVals)
	} else {
		winner, err = determineMeta(explicit, bases)
		if err != nil {
			return nil, err
		}
		ns, err = prepareNamespace(winner, name, bases, kwNames, kwVals)
	}
	if err != nil {
		return nil, err
	}
	b := &ClassBuilder{meta: winner, callable: callable, name: name, qual: qual, bases: bases, ns: ns, kwNames: kwNames, kwVals: kwVals}
	if err := b.Set("__module__", NewStr(module)); err != nil {
		return nil, err
	}
	if err := b.Set("__qualname__", NewStr(strings.TrimPrefix(qual, module+"."))); err != nil {
		return nil, err
	}
	if err := b.Set("__firstlineno__", NewInt(int64(firstLine))); err != nil {
		return nil, err
	}
	if doc != nil {
		if err := b.Set("__doc__", doc); err != nil {
			return nil, err
		}
	}
	return b, nil
}

// prepareNamespace builds the namespace the class body populates. The default
// metatype and a metaclass without __prepare__ get a fresh dict, matching
// type.__prepare__; a user __prepare__ is resolved on the metaclass MRO and
// called with (name, bases) plus the class keywords, a classmethod binding
// the metaclass first the way attribute access would. The result must be a
// mapping, checked with the probed type.__new__-adjacent wording.
func prepareNamespace(winner *classObject, name string, bases []Object, kwNames []string, kwVals []Object) (Object, error) {
	if winner == typeClass {
		return NewDict(nil, nil)
	}
	prep, ok := winner.lookup("__prepare__")
	if !ok {
		return NewDict(nil, nil)
	}
	args := []Object{NewStr(name), metaBasesTuple(bases)}
	switch p := prep.(type) {
	case *classmethodObject:
		prep = p.fn
		args = append([]Object{winner}, args...)
	case *staticmethodObject:
		prep = p.fn
	}
	ns, err := CallKw(prep, args, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	if !nsIsMapping(ns) {
		return nil, Raise(TypeError, "%s.__prepare__() must return a mapping, not %s", winner.name, ns.TypeName())
	}
	return ns, nil
}

// prepareCallable builds the namespace for a non-type callable metaclass:
// __prepare__ resolves as an ordinary attribute on the callable itself (an
// instance's method binds it, a missing attribute means the default dict) and
// the mapping check spells the winner as the literal <metaclass>, the wording
// __build_class__ uses when the metaclass is not a type.
func prepareCallable(callable Object, name string, bases []Object, kwNames []string, kwVals []Object) (Object, error) {
	prep, err := LoadAttr(callable, "__prepare__")
	if err != nil {
		if isAttrError(err) {
			return NewDict(nil, nil)
		}
		return nil, err
	}
	ns, err := CallKw(prep, []Object{NewStr(name), metaBasesTuple(bases)}, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	if !nsIsMapping(ns) {
		return nil, Raise(TypeError, "<metaclass>.__prepare__() must return a mapping, not %s", ns.TypeName())
	}
	return ns, nil
}

// nsIsMapping mirrors the PyMapping_Check gate on the __prepare__ result: a
// dict passes, and so does an instance whose class defines __getitem__.
func nsIsMapping(o Object) bool {
	switch x := o.(type) {
	case *dictObject:
		return true
	case *instanceObject:
		_, ok := x.cls.lookup("__getitem__")
		return ok
	}
	return false
}

// Set writes one namespace binding through the item protocol, so a custom
// mapping's __setitem__ observes it.
func (b *ClassBuilder) Set(name string, v Object) error {
	return SetItem(b.ns, NewStr(name), v)
}

// Load reads a name the class body may have bound in the namespace. ok is
// false when the namespace holds no such key, the class body's cue to fall
// through to the enclosing module and builtin scopes, matching the LOAD_NAME
// lookup order. The default dict namespace answers directly; a custom
// __prepare__ mapping is read through the item protocol so its __getitem__
// observes the read, and a KeyError there is the same not-found fall-through.
func (b *ClassBuilder) Load(name string) (Object, bool, error) {
	if d, ok := b.ns.(*dictObject); ok {
		return d.lookup(NewStr(name))
	}
	v, err := GetItem(b.ns, NewStr(name))
	if err != nil {
		if e, ok := err.(*Exception); ok && e.Kind == KeyError {
			return nil, false, nil
		}
		return nil, false, err
	}
	return v, true, nil
}

// Delete removes a name the class body bound, the STORE_NAME namespace's
// DELETE_NAME. An except handler's as-name is bound on entry and deleted here
// on exit, so a later read of it in the body falls through and raises
// NameError. A missing key is a NameError the caller surfaces at the delete
// site; the default dict namespace reports it directly, and a custom
// __prepare__ mapping reports whatever its __delitem__ raises.
func (b *ClassBuilder) Delete(name string) error {
	if d, ok := b.ns.(*dictObject); ok {
		_, found, err := d.delete(NewStr(name))
		if err != nil {
			return err
		}
		if !found {
			return Raise(NameError, "name '%s' is not defined", name)
		}
		return nil
	}
	return DelItem(b.ns, NewStr(name))
}

// Finish writes the trailing synthesized member and runs the metaclass call.
// staticAttrs are the names the class's methods assign on self, already
// sorted; CPython's compiler synthesizes them as __static_attributes__ after
// the body bindings. The default metatype keeps the direct build; a user
// metaclass receives the namespace object itself.
func (b *ClassBuilder) Finish(staticAttrs []string) (Object, error) {
	elts := make([]Object, len(staticAttrs))
	for i, s := range staticAttrs {
		elts[i] = NewStr(s)
	}
	if err := b.Set("__static_attributes__", NewTuple(elts)); err != nil {
		return nil, err
	}
	// A non-type callable, and a plain class that won determination without
	// deriving from type, both just get called with (name, bases, ns) and the
	// class keywords: no __new__/__init__ metaclass protocol, and the return
	// value is the class binding whatever it is.
	if b.callable != nil {
		return CallKw(b.callable, []Object{NewStr(b.name), metaBasesTuple(b.bases), b.ns}, b.kwNames, b.kwVals)
	}
	if b.meta == typeClass {
		names, vals, err := nsBodyItems(b.ns)
		if err != nil {
			return nil, err
		}
		return newClassCore(nil, b.name, b.qual, b.bases, names, vals, b.kwNames, b.kwVals)
	}
	if !b.meta.isMeta {
		return CallKw(b.meta, []Object{NewStr(b.name), metaBasesTuple(b.bases), b.ns}, b.kwNames, b.kwVals)
	}
	return callMetaclass(b.meta, b.name, b.qual, b.bases, b.ns, b.kwNames, b.kwVals)
}

// nsBodyItems unpacks a namespace into the ordered name/value pairs
// newClassCore slots into the class dict. Only a real dict unpacks, the same
// requirement type.__new__ enforces with its argument-3 wording; a custom
// mapping reaches this point only when a metaclass forwards it to the default
// __new__, exactly where CPython raises too. __qualname__ is dropped the way
// type.__new__ pops it, since the class object carries it structurally, and a
// non-str key cannot slot a name so it is skipped, the documented edge
// typeNewCore shares.
func nsBodyItems(ns Object) ([]string, []Object, error) {
	d, ok := ns.(*dictObject)
	if !ok {
		return nil, nil, Raise(TypeError, "type.__new__() argument 3 must be dict, not %s", ns.TypeName())
	}
	var names []string
	var vals []Object
	for _, e := range d.entries {
		k, ok := e.key.(*strObject)
		if !ok {
			continue
		}
		if k.v == "__qualname__" {
			continue
		}
		names = append(names, k.v)
		vals = append(vals, e.val)
	}
	return names, vals, nil
}
