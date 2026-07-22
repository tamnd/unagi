package objects

// Subclassing a builtin type. A class statement may name a builtin type such as
// dict among its bases; that base is not a classObject, so it never joins the
// MRO. Instead the class records it in builtinBase and its instances carry the
// matching payload, a dictData store for a dict subclass. The mapping protocol
// and the inherited dict methods route to that store here, but only when the
// class did not override the operation, so a user __setitem__ or keys() still
// wins the way it does in CPython.
//
// This is the dict layer of the builtin-subclassing frontier; the int, str and
// tuple value subclasses (enum's IntEnum and StrEnum, codecs' CodecInfo) are
// their own slices.

// builtinBaseName reports the builtin type a base expression names when that
// type can be subclassed, so newClassCore can record the layout instead of
// rejecting the base. dict, int, str and tuple are supported; every other
// builtin still reaches the bases-must-be-types path.
func builtinBaseName(b Object) (string, bool) {
	if f, ok := b.(*funcObject); ok {
		switch f.name {
		case "dict", "int", "str", "tuple", "classmethod", "staticmethod", "property":
			return f.name, true
		case "local":
			// threading.local is exposed as the builtin `local`; a subclass takes
			// the per-thread attribute layout rather than the ordinary instance
			// dict. It carries no value payload, so construction never calls the
			// base function; instantiateLocal builds the per-thread store instead.
			return "local", true
		}
	}
	// types.GenericAlias is a type object rather than a builtin function, the
	// value `from types import GenericAlias` binds. _collections_abc subclasses
	// it as _CallableGenericAlias, so it is a value base whose payload is a
	// genericAliasObject; it carries no builtinBaseFn, so every construction site
	// special-cases the name.
	if t, ok := b.(*typeObject); ok && t.name == "types.GenericAlias" {
		return "types.GenericAlias", true
	}
	return "", false
}

// isDescriptorBase reports whether a builtin base wraps a callable as a
// descriptor, the classmethod, staticmethod and property layer of builtin
// subclassing. A subclass instance holds the wrapped descriptor as its payload
// and delegates the descriptor protocol to it, the way abc's abstractclassmethod
// and abstractproperty do.
func isDescriptorBase(name string) bool {
	return name == "classmethod" || name == "staticmethod" || name == "property"
}

// descriptorPayload unwraps a classmethod, staticmethod or property subclass
// instance to the builtin descriptor it wraps, so the descriptor protocol runs
// on that payload. It applies only when the subclass adds no descriptor hook of
// its own; a subclass that defines __get__, __set__ or __delete__ keeps the
// ordinary user-descriptor path. ok is false for anything that is not such an
// instance, so a plain object still dispatches on its own type.
func descriptorPayload(v Object) (Object, bool) {
	inst, ok := v.(*instanceObject)
	if !ok || !isDescriptorBase(inst.cls.builtinBase) || inst.builtinData == nil {
		return nil, false
	}
	for _, hook := range []string{"__get__", "__set__", "__delete__"} {
		if _, ok := inst.cls.lookup(hook); ok {
			return nil, false
		}
	}
	return inst.builtinData, true
}

// builtinUnwrap returns the immutable builtin payload a value subclass instance
// wraps, the int an int subclass holds. ok is false for any other object,
// including a dict subclass whose store is not a scalar value. An operator uses
// it to compute on the underlying builtin after an override lookup misses.
func builtinUnwrap(o Object) (Object, bool) {
	inst, ok := o.(*instanceObject)
	if !ok || inst.builtinData == nil {
		return nil, false
	}
	return inst.builtinData, true
}

// BuiltinValue exposes builtinUnwrap to callers in other packages, so a
// conversion such as int(x) or an index can read the payload of a value subclass
// instance. ok is false for every object that is not such an instance.
func BuiltinValue(o Object) (Object, bool) {
	return builtinUnwrap(o)
}

// builtinUnary runs a unary operator on a value subclass instance: a class
// override wins (the shape IntFlag's __invert__ takes), otherwise the payload
// carries the operation and returns the plain builtin result. ok is false for an
// instance that is not value-backed, so the caller keeps its own type error.
func builtinUnary(o Object, dunder string, apply func(Object) (Object, error)) (Object, bool, error) {
	inst, ok := o.(*instanceObject)
	if !ok {
		return nil, false, nil
	}
	if _, has := inst.cls.lookup(dunder); has {
		res, defined, err := instanceSpecial(inst, dunder)
		if defined {
			return res, true, err
		}
	}
	if inst.builtinData == nil {
		return nil, false, nil
	}
	r, err := apply(inst.builtinData)
	return r, true, err
}

// strSubclassMethods names the str methods a str subclass inherits, the set an
// attribute read on a str-backed instance hands back as a callable bound to the
// payload when the class defines no override. It mirrors the str method surface
// CallMethod dispatches, so a StrEnum member answers upper(), split() and the
// rest from its underlying str.
var strSubclassMethods = map[string]bool{
	"capitalize": true, "casefold": true, "center": true, "count": true,
	"encode": true, "endswith": true, "expandtabs": true, "find": true,
	"format": true, "format_map": true, "index": true, "isalnum": true,
	"isalpha": true, "isascii": true, "isdecimal": true, "isdigit": true,
	"isidentifier": true, "islower": true, "isnumeric": true, "isprintable": true,
	"isspace": true, "istitle": true, "isupper": true, "join": true,
	"ljust": true, "lower": true, "lstrip": true, "maketrans": true,
	"partition": true, "removeprefix": true, "removesuffix": true, "replace": true,
	"rfind": true, "rindex": true, "rjust": true, "rpartition": true,
	"rsplit": true, "rstrip": true, "split": true, "splitlines": true,
	"startswith": true, "strip": true, "swapcase": true, "title": true,
	"translate": true, "upper": true, "zfill": true,
}

// tupleSubclassMethods names the tuple methods a tuple subclass inherits, the
// set LoadAttr hands back as callables bound to the payload when the class
// defines no override. tuple's method surface is just count and index, so a
// CodecInfo answers those from its underlying tuple.
var tupleSubclassMethods = map[string]bool{
	"count": true, "index": true,
}

// valueSubclassAttr resolves an inherited builtin method on a value subclass
// instance, returning a callable bound to the payload. ok is false when the
// instance is not value-backed or the name is not one of the builtin's methods,
// so LoadAttr keeps its ordinary AttributeError. A user override lives in the
// class dict and is found before this fallback runs, so it never shadows one.
// str and tuple carry a method surface here; the int payload exposes no methods
// in this tier, matching how the int subclass slice left them.
func valueSubclassAttr(x *instanceObject, name string) (Object, bool) {
	v, ok := builtinUnwrap(x)
	if !ok {
		return nil, false
	}
	switch p := v.(type) {
	case *strObject:
		if !strSubclassMethods[name] {
			return nil, false
		}
	case *tupleObject:
		if p.named != nil {
			// A namedtuple subclass instance answers its field names and the
			// namedtuple helpers off the tuple payload, so t.field, t._replace and
			// t._asdict resolve before the plain tuple method surface.
			if r, ok := namedInstanceAttr(x.cls, p, name); ok {
				return r, true
			}
		}
		if !tupleSubclassMethods[name] {
			return nil, false
		}
	case *genericAliasObject:
		// A GenericAlias subclass inherits __origin__, __args__ and
		// __parameters__ as plain attribute reads off its wrapped generic, the
		// values _CallableGenericAlias reads through self.__args__.
		val, err := genericAliasLoadAttr(p, name)
		if err != nil {
			return nil, false
		}
		return val, true
	default:
		return nil, false
	}
	fn := func(args []Object) (Object, error) { return CallMethod(v, name, args) }
	return NewFunc(name, -1, fn), true
}

// InstanceOverride runs a special method a user class defines on an instance,
// for a caller in another package that must try an override before its own
// builtin path. ok is false when o is not an instance or its class defines no
// such method, so the caller keeps its default behavior.
func InstanceOverride(o Object, name string, args ...Object) (Object, bool, error) {
	inst, ok := o.(*instanceObject)
	if !ok {
		return nil, false, nil
	}
	return instanceSpecial(inst, name, args...)
}

// dictBacked returns the mapping store of a dict subclass instance, ok true only
// when the instance's class derives from dict and the store is allocated. The
// operator and method sites use it as the fallback after an override lookup
// misses.
func dictBacked(x *instanceObject) (*dictObject, bool) {
	if x.cls.builtinBase == "dict" && x.dictData != nil {
		return x.dictData, true
	}
	return nil, false
}

// dictInit seeds a dict subclass instance's store the way dict(...) does: at
// most one positional argument, a mapping merged by its keys or any other value
// read as an iterable of key-value pairs, then the keyword items in order. It is
// what runs when a dict subclass inherits dict.__init__ rather than overriding
// it.
func dictInit(d *dictObject, pos []Object, kwNames []string, kwVals []Object) error {
	if len(pos) > 1 {
		return Raise(TypeError, "dict expected at most 1 argument, got %d", len(pos))
	}
	if len(pos) == 1 {
		if isMappingArg(pos[0]) {
			if err := d.mergeMapping(pos[0]); err != nil {
				return err
			}
		} else if err := dictUpdate(d, pos[0]); err != nil {
			return err
		}
	}
	for i, k := range kwNames {
		if err := d.set(NewStr(k), kwVals[i]); err != nil {
			return err
		}
	}
	return nil
}

// isMappingArg reports whether dict(...) should treat src as a mapping to merge
// by keys rather than an iterable of pairs. A real dict qualifies, and so does
// anything exposing a keys() method, the PyMapping_Keys probe CPython uses.
func isMappingArg(src Object) bool {
	if _, ok := src.(*dictObject); ok {
		return true
	}
	if _, err := LoadAttr(src, "keys"); err == nil {
		return true
	}
	return false
}

// dictSubclassMethods names the dict methods a dict subclass inherits, the set
// LoadAttr hands back as bound callables when the class defines no override.
var dictSubclassMethods = map[string]bool{
	"get": true, "pop": true, "popitem": true, "setdefault": true,
	"keys": true, "values": true, "items": true, "clear": true,
	"copy": true, "update": true, "fromkeys": true,
}

// mappingDunders names the dict operators a dict subclass reaches through
// super(), the cooperative-walk fall-through that lands on the builtin base.
var mappingDunders = map[string]bool{
	"__init__": true, "__setitem__": true, "__getitem__": true,
	"__delitem__": true, "__len__": true, "__contains__": true,
}

// builtinBaseCall runs a dict subclass's inherited builtin method when the
// cooperative super() walk falls past the last user class onto the recorded
// builtin base. self is the instance super was bound to; ok is false when self
// is not dict-backed or the name is not one the dict base provides, so the
// caller falls through to the object-root defaults. kwNames and kwVals carry the
// keyword items super().__init__ and super().update accept.
func builtinBaseCall(self Object, name string, pos []Object, kwNames []string, kwVals []Object) (Object, bool, error) {
	inst, ok := self.(*instanceObject)
	if !ok {
		return nil, false, nil
	}
	d, backed := dictBacked(inst)
	if !backed {
		return nil, false, nil
	}
	switch name {
	case "__init__":
		return None, true, dictInit(d, pos, kwNames, kwVals)
	case "__setitem__":
		if len(pos) != 2 {
			return nil, true, Raise(TypeError, "__setitem__ expected 2 arguments, got %d", len(pos))
		}
		return None, true, d.set(pos[0], pos[1])
	case "__getitem__":
		if len(pos) != 1 {
			return nil, true, Raise(TypeError, "__getitem__ expected 1 argument, got %d", len(pos))
		}
		v, err := d.get(pos[0])
		return v, true, err
	case "__delitem__":
		if len(pos) != 1 {
			return nil, true, Raise(TypeError, "__delitem__ expected 1 argument, got %d", len(pos))
		}
		_, found, err := d.delete(pos[0])
		if err != nil {
			return nil, true, err
		}
		if !found {
			return nil, true, NewException(KeyError, []Object{pos[0]})
		}
		return None, true, nil
	case "__len__":
		return NewInt(int64(len(d.entries))), true, nil
	case "__contains__":
		if len(pos) != 1 {
			return nil, true, Raise(TypeError, "__contains__ expected 1 argument, got %d", len(pos))
		}
		_, found, err := d.lookup(pos[0])
		if err != nil {
			return nil, true, err
		}
		return NewBool(found), true, nil
	}
	if dictSubclassMethods[name] {
		if name == "update" {
			return None, true, superUpdate(d, pos, kwNames, kwVals)
		}
		r, err := dictMethod(d, name, pos)
		return r, true, err
	}
	return nil, false, nil
}

// superUpdate is dict.update reached through super(): an optional mapping or
// pair iterable followed by keyword items, matching dict.update(src, **kw).
func superUpdate(d *dictObject, pos []Object, kwNames []string, kwVals []Object) error {
	if len(pos) > 1 {
		return Raise(TypeError, "update expected at most 1 argument, got %d", len(pos))
	}
	if len(pos) == 1 {
		if err := dictUpdate(d, pos[0]); err != nil {
			return err
		}
	}
	for i, k := range kwNames {
		if err := d.set(NewStr(k), kwVals[i]); err != nil {
			return err
		}
	}
	return nil
}

// builtinBaseAttr resolves a builtin-base method reached through a bare super()
// attribute read, returning a callable bound to the instance store. It backs the
// `f = super().update` shape, where the call comes later.
func builtinBaseAttr(self Object, name string) (Object, bool) {
	inst, ok := self.(*instanceObject)
	if !ok {
		return nil, false
	}
	if _, backed := dictBacked(inst); !backed {
		return nil, false
	}
	if !dictSubclassMethods[name] && !mappingDunders[name] {
		return nil, false
	}
	fn := func(args []Object) (Object, error) {
		r, handled, err := builtinBaseCall(self, name, args, nil, nil)
		if !handled {
			return nil, Raise(AttributeError, "'super' object has no attribute '%s'", name)
		}
		return r, err
	}
	return NewFunc(name, -1, fn), true
}

// dictSubclassAttr resolves an inherited dict method on a dict subclass
// instance, returning a callable bound to the instance's store. ok is false for
// a name that is not an inherited dict method, so LoadAttr keeps its ordinary
// AttributeError. A user override lives in the class dict and is found before
// this fallback runs, so it never shadows one.
func dictSubclassAttr(x *instanceObject, name string) (Object, bool) {
	d, backed := dictBacked(x)
	if !backed || !dictSubclassMethods[name] {
		return nil, false
	}
	fn := func(args []Object) (Object, error) { return dictMethod(d, name, args) }
	return NewFunc(name, -1, fn), true
}
