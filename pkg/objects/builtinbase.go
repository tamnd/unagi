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
