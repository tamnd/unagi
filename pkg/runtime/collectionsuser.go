package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// UserDict, UserList and UserString are the pure-Python wrapper classes the
// collections package defines on top of dict, list and str. They cannot ride in
// on the vendored collections/__init__.py, since that module also builds
// namedtuple through eval and a dynamic type() the AOT world does not run, so
// the native module owns them here the way it owns Counter and namedtuple.
//
// Each subclasses the matching abstract base from _collections_abc (UserDict a
// MutableMapping, UserList a MutableSequence, UserString a Sequence) and fills
// in the abstract methods over a self.data attribute, so the mixin surface
// (update, get, pop, keys, extend, index and the rest) comes from the base by
// the ordinary MRO, exactly as the CPython classes get it. This is what lets
// import pprint finish: it keys its dispatch table on
// collections.UserDict.__repr__ and the two siblings, which now exist.

var (
	userDictClass   objects.Object
	userListClass   objects.Object
	userStringClass objects.Object
)

// buildUserClasses builds UserDict, UserList and UserString over the abstract
// bases the given _collections_abc module carries, storing them into the
// collections module and the global type table. It is best effort: without the
// compiled floor the abc module is absent, so the classes are simply left off,
// the same way the collections.abc alias is.
func buildUserClasses(m *objects.Module, abc objects.Object) error {
	mutableMapping, err := objects.LoadAttr(abc, "MutableMapping")
	if err != nil {
		return err
	}
	mutableSequence, err := objects.LoadAttr(abc, "MutableSequence")
	if err != nil {
		return err
	}
	sequence, err := objects.LoadAttr(abc, "Sequence")
	if err != nil {
		return err
	}

	userDictClass, err = buildUserDict(mutableMapping)
	if err != nil {
		return err
	}
	userListClass, err = buildUserList(mutableSequence)
	if err != nil {
		return err
	}
	userStringClass, err = buildUserString(sequence)
	if err != nil {
		return err
	}

	for _, e := range []struct {
		name string
		v    objects.Object
	}{
		{"UserDict", userDictClass},
		{"UserList", userListClass},
		{"UserString", userStringClass},
	} {
		if err := objects.StoreAttr(m, e.name, e.v); err != nil {
			return err
		}
		// type(x) resolves the type object out of the global builtin table by its
		// dotted name, matching the deque/defaultdict registration above.
		builtins["collections."+e.name] = e.v
	}
	return nil
}

// userData reads the self.data attribute the wrapper holds its payload in.
func userData(self objects.Object) (objects.Object, error) {
	return objects.LoadAttr(self, "data")
}

// userSetData stores the self.data attribute.
func userSetData(self, v objects.Object) error {
	return objects.StoreAttr(self, "data", v)
}

// isUser reports whether obj is an instance of the given wrapper class, the
// isinstance(other, UserList) test the arithmetic and comparison methods make.
func isUser(obj, cls objects.Object) bool {
	if cls == nil {
		return false
	}
	r, err := objects.IsInstance(obj, cls)
	return err == nil && r == objects.True
}

// userReconstruct calls self.__class__(arg), the way the wrapper methods build a
// result of the same (possibly subclass) type.
func userReconstruct(self, arg objects.Object) (objects.Object, error) {
	cls, ok := objects.ClassOf(self)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "cannot determine class")
	}
	return objects.Call(cls, []objects.Object{arg})
}

// ---------------------------------------------------------------------------
// UserDict
// ---------------------------------------------------------------------------

func buildUserDict(base objects.Object) (objects.Object, error) {
	names := []string{
		"__init__", "__len__", "__getitem__", "__setitem__", "__delitem__",
		"__iter__", "__contains__", "get", "__repr__",
		"__or__", "__ror__", "__ior__", "copy", "fromkeys",
	}
	vals := []objects.Object{
		objects.NewMethodKw("__init__", userDictInit),
		objects.NewMethod("__len__", 1, userDictLen),
		objects.NewMethod("__getitem__", 2, userDictGetItem),
		objects.NewMethod("__setitem__", 3, userDictSetItem),
		objects.NewMethod("__delitem__", 2, userDictDelItem),
		objects.NewMethod("__iter__", 1, userDictIter),
		objects.NewMethod("__contains__", 2, userDictContains),
		objects.NewMethodKw("get", userDictGet),
		objects.NewMethod("__repr__", 1, userReprData),
		objects.NewMethod("__or__", 2, userDictOr),
		objects.NewMethod("__ror__", 2, userDictROr),
		objects.NewMethod("__ior__", 2, userDictIOr),
		objects.NewMethod("copy", 1, userDictCopy),
		objects.NewClassMethod(objects.NewMethodKw("fromkeys", userDictFromKeys)),
	}
	return objects.NewClass("UserDict", "collections.UserDict",
		[]objects.Object{base}, names, vals, nil, nil)
}

func userDictInit(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 1 {
		return nil, objects.Raise(objects.TypeError, "__init__ needs self")
	}
	self := pos[0]
	empty, err := objects.NewDict(nil, nil)
	if err != nil {
		return nil, err
	}
	if err := userSetData(self, empty); err != nil {
		return nil, err
	}
	if len(pos) > 1 && pos[1] != objects.None {
		if _, err := objects.CallMethod(self, "update", []objects.Object{pos[1]}); err != nil {
			return nil, err
		}
	}
	if len(kwNames) > 0 {
		keys := make([]objects.Object, len(kwNames))
		for i, n := range kwNames {
			keys[i] = objects.NewStr(n)
		}
		kw, err := objects.NewDict(keys, kwVals)
		if err != nil {
			return nil, err
		}
		if _, err := objects.CallMethod(self, "update", []objects.Object{kw}); err != nil {
			return nil, err
		}
	}
	return objects.None, nil
}

func userDictLen(args []objects.Object) (objects.Object, error) {
	data, err := userData(args[0])
	if err != nil {
		return nil, err
	}
	n, err := objects.Len(data)
	if err != nil {
		return nil, err
	}
	return objects.NewInt(int64(n)), nil
}

func userDictGetItem(args []objects.Object) (objects.Object, error) {
	self, key := args[0], args[1]
	data, err := userData(self)
	if err != nil {
		return nil, err
	}
	in, err := objects.Contains(data, key)
	if err != nil {
		return nil, err
	}
	if in == objects.True {
		return objects.GetItem(data, key)
	}
	// dict-with-__missing__ behavior: a subclass may define __missing__.
	if cls, ok := objects.ClassOf(self); ok {
		if missing, err := objects.LoadAttr(cls, "__missing__"); err == nil {
			return objects.Call(missing, []objects.Object{self, key})
		}
	}
	return nil, objects.Raise(objects.KeyError, "%s", objects.Repr(key))
}

func userDictSetItem(args []objects.Object) (objects.Object, error) {
	data, err := userData(args[0])
	if err != nil {
		return nil, err
	}
	if err := objects.SetItem(data, args[1], args[2]); err != nil {
		return nil, err
	}
	return objects.None, nil
}

func userDictDelItem(args []objects.Object) (objects.Object, error) {
	data, err := userData(args[0])
	if err != nil {
		return nil, err
	}
	if err := objects.DelItem(data, args[1]); err != nil {
		return nil, err
	}
	return objects.None, nil
}

func userDictIter(args []objects.Object) (objects.Object, error) {
	data, err := userData(args[0])
	if err != nil {
		return nil, err
	}
	return objects.CallMethod(data, "__iter__", nil)
}

func userDictContains(args []objects.Object) (objects.Object, error) {
	data, err := userData(args[0])
	if err != nil {
		return nil, err
	}
	in, err := objects.Contains(data, args[1])
	if err != nil {
		return nil, err
	}
	return in, nil
}

func userDictGet(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	self := pos[0]
	var key, def objects.Object
	if len(pos) > 1 {
		key = pos[1]
	}
	def = objects.None
	if len(pos) > 2 {
		def = pos[2]
	}
	for i, n := range kwNames {
		switch n {
		case "key":
			key = kwVals[i]
		case "default":
			def = kwVals[i]
		}
	}
	in, err := objects.CallMethod(self, "__contains__", []objects.Object{key})
	if err != nil {
		return nil, err
	}
	if in == objects.True {
		return objects.GetItem(self, key)
	}
	return def, nil
}

// userReprData reprs self.data, shared by all three wrappers.
func userReprData(args []objects.Object) (objects.Object, error) {
	data, err := userData(args[0])
	if err != nil {
		return nil, err
	}
	return objects.NewStr(objects.Repr(data)), nil
}

// userDictMerge builds self.__class__ from self.data merged with other, honoring
// that other may itself be a UserDict (unwrapped to its data) or a plain dict.
func userDictMerge(self, other objects.Object, selfFirst bool) (objects.Object, error) {
	data, err := userData(self)
	if err != nil {
		return nil, err
	}
	var rhs objects.Object
	if isUser(other, userDictClass) {
		if rhs, err = userData(other); err != nil {
			return nil, err
		}
	} else {
		rhs = other
	}
	var merged objects.Object
	if selfFirst {
		merged, err = objects.BitOr(data, rhs)
	} else {
		merged, err = objects.BitOr(rhs, data)
	}
	if err != nil {
		return nil, err
	}
	return userReconstruct(self, merged)
}

func userDictOr(args []objects.Object) (objects.Object, error) {
	other := args[1]
	if !isUser(other, userDictClass) && !isDict(other) {
		return objects.NotImplemented, nil
	}
	return userDictMerge(args[0], other, true)
}

func userDictROr(args []objects.Object) (objects.Object, error) {
	other := args[1]
	if !isUser(other, userDictClass) && !isDict(other) {
		return objects.NotImplemented, nil
	}
	return userDictMerge(args[0], other, false)
}

func userDictIOr(args []objects.Object) (objects.Object, error) {
	self, other := args[0], args[1]
	data, err := userData(self)
	if err != nil {
		return nil, err
	}
	rhs := other
	if isUser(other, userDictClass) {
		if rhs, err = userData(other); err != nil {
			return nil, err
		}
	}
	merged, err := objects.BitOr(data, rhs)
	if err != nil {
		return nil, err
	}
	if err := userSetData(self, merged); err != nil {
		return nil, err
	}
	return self, nil
}

func userDictCopy(args []objects.Object) (objects.Object, error) {
	self := args[0]
	data, err := userData(self)
	if err != nil {
		return nil, err
	}
	dup, err := objects.CallMethod(data, "copy", nil)
	if err != nil {
		return nil, err
	}
	return userReconstruct(self, dup)
}

func userDictFromKeys(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	// classmethod: pos[0] is the class, pos[1] the iterable, pos[2] the value.
	cls := pos[0]
	var iterable, value objects.Object
	if len(pos) > 1 {
		iterable = pos[1]
	}
	value = objects.None
	if len(pos) > 2 {
		value = pos[2]
	}
	inst, err := objects.Call(cls, nil)
	if err != nil {
		return nil, err
	}
	it, err := objects.Iter(iterable)
	if err != nil {
		return nil, err
	}
	for {
		key, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		if _, err := objects.CallMethod(inst, "__setitem__", []objects.Object{key, value}); err != nil {
			return nil, err
		}
	}
	return inst, nil
}

// isDict reports whether o is a plain dict, the isinstance(other, dict) guard.
func isDict(o objects.Object) bool {
	return o.TypeName() == "dict"
}
