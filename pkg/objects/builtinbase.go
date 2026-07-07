package objects

// Subclassing a builtin type. A class statement may name a builtin type such as
// dict among its bases; that base is not a classObject, so it never joins the
// MRO. Instead the class records it in builtinBase and its instances carry the
// matching payload, a dictData store for a dict subclass. The mapping protocol
// and the inherited dict methods route to that store here, but only when the
// class did not override the operation, so a user __setitem__ or keys() still
// wins the way it does in CPython.
//
// This is the dict layer of the builtin-subclassing frontier; the int and str
// value subclasses that enum's IntEnum and StrEnum need are their own slices.

// builtinBaseName reports the builtin type a base expression names when that
// type can be subclassed, so newClassCore can record the layout instead of
// rejecting the base. Only dict is supported so far; every other builtin still
// reaches the bases-must-be-types path.
func builtinBaseName(b Object) (string, bool) {
	if f, ok := b.(*funcObject); ok && f.name == "dict" {
		return "dict", true
	}
	return "", false
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
