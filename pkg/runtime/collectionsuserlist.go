package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// UserList wraps a list in self.data and subclasses MutableSequence, so append,
// extend, reverse, pop, remove and __iadd__ come from the base while this file
// supplies the abstract methods, the comparisons, the arithmetic, and the small
// set of list methods the mixin does not carry.

func buildUserList(base objects.Object) (objects.Object, error) {
	names := []string{
		"__init__", "__repr__",
		"__lt__", "__le__", "__eq__", "__gt__", "__ge__",
		"__contains__", "__len__", "__getitem__", "__setitem__", "__delitem__",
		"__add__", "__radd__", "__iadd__", "__mul__", "__rmul__", "__imul__",
		"append", "insert", "pop", "remove", "clear", "copy",
		"count", "index", "reverse", "sort", "extend",
	}
	vals := []objects.Object{
		objects.NewMethod("__init__", -1, userListInit),
		objects.NewMethod("__repr__", 1, userReprData),
		userListCmp("__lt__", objects.OpLt),
		userListCmp("__le__", objects.OpLe),
		userListCmp("__eq__", objects.OpEq),
		userListCmp("__gt__", objects.OpGt),
		userListCmp("__ge__", objects.OpGe),
		objects.NewMethod("__contains__", 2, userListContains),
		objects.NewMethod("__len__", 1, userDictLen),
		objects.NewMethod("__getitem__", 2, userListGetItem),
		objects.NewMethod("__setitem__", 3, userListSetItem),
		objects.NewMethod("__delitem__", 2, userListDelItem),
		objects.NewMethod("__add__", 2, userListAdd),
		objects.NewMethod("__radd__", 2, userListRAdd),
		objects.NewMethod("__iadd__", 2, userListIAdd),
		objects.NewMethod("__mul__", 2, userListMul),
		objects.NewMethod("__rmul__", 2, userListMul),
		objects.NewMethod("__imul__", 2, userListIMul),
		objects.NewMethod("append", 2, userListAppend),
		objects.NewMethod("insert", 3, userListInsert),
		objects.NewMethodKw("pop", userListPop),
		objects.NewMethod("remove", 2, userListRemove),
		objects.NewMethod("clear", 1, userListClear),
		objects.NewMethod("copy", 1, userListCopy),
		objects.NewMethod("count", 2, userListCount),
		objects.NewMethodKw("index", userListIndex),
		objects.NewMethod("reverse", 1, userListReverse),
		objects.NewMethodKw("sort", userListSort),
		objects.NewMethod("extend", 2, userListExtend),
	}
	return objects.NewClass("UserList", "collections.UserList",
		[]objects.Object{base}, names, vals, nil, nil)
}

func userListInit(args []objects.Object) (objects.Object, error) {
	self := args[0]
	if err := userSetData(self, objects.NewList(nil)); err != nil {
		return nil, err
	}
	if len(args) < 2 || args[1] == objects.None {
		return objects.None, nil
	}
	initlist := args[1]
	// A UserList seeds from a copy of the other's data; anything else is drawn
	// through list() so an arbitrary iterable works.
	if isUser(initlist, userListClass) {
		other, err := userData(initlist)
		if err != nil {
			return nil, err
		}
		dup, err := objects.CallMethod(other, "copy", nil)
		if err != nil {
			return nil, err
		}
		return objects.None, userSetData(self, dup)
	}
	elts, err := materialize(initlist)
	if err != nil {
		return nil, err
	}
	return objects.None, userSetData(self, objects.NewList(elts))
}

// userListCast unwraps a UserList operand to its data, leaving other operands as
// they are, so a comparison lines up the wrapped list against the raw one.
func userListCast(other objects.Object) (objects.Object, error) {
	if isUser(other, userListClass) {
		return userData(other)
	}
	return other, nil
}

func userListCmp(name string, op objects.CmpOp) objects.Object {
	return objects.NewMethod(name, 2, func(args []objects.Object) (objects.Object, error) {
		data, err := userData(args[0])
		if err != nil {
			return nil, err
		}
		rhs, err := userListCast(args[1])
		if err != nil {
			return nil, err
		}
		return objects.Compare(op, data, rhs)
	})
}

func userListContains(args []objects.Object) (objects.Object, error) {
	data, err := userData(args[0])
	if err != nil {
		return nil, err
	}
	return objects.Contains(data, args[1])
}

func userListGetItem(args []objects.Object) (objects.Object, error) {
	self, i := args[0], args[1]
	data, err := userData(self)
	if err != nil {
		return nil, err
	}
	if i.TypeName() == "slice" {
		part, err := objects.GetItem(data, i)
		if err != nil {
			return nil, err
		}
		return userReconstruct(self, part)
	}
	return objects.GetItem(data, i)
}

func userListSetItem(args []objects.Object) (objects.Object, error) {
	data, err := userData(args[0])
	if err != nil {
		return nil, err
	}
	if err := objects.SetItem(data, args[1], args[2]); err != nil {
		return nil, err
	}
	return objects.None, nil
}

func userListDelItem(args []objects.Object) (objects.Object, error) {
	data, err := userData(args[0])
	if err != nil {
		return nil, err
	}
	if err := objects.DelItem(data, args[1]); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// userListConcat builds self.__class__ from self.data concatenated with other,
// unwrapping a UserList operand and drawing any other iterable through list().
func userListConcat(self, other objects.Object, selfFirst bool) (objects.Object, error) {
	data, err := userData(self)
	if err != nil {
		return nil, err
	}
	var rhs objects.Object
	if isUser(other, userListClass) {
		if rhs, err = userData(other); err != nil {
			return nil, err
		}
	} else if other.TypeName() == "list" {
		rhs = other
	} else {
		elts, err := materialize(other)
		if err != nil {
			return nil, err
		}
		rhs = objects.NewList(elts)
	}
	var sum objects.Object
	if selfFirst {
		sum, err = objects.Add(data, rhs)
	} else {
		sum, err = objects.Add(rhs, data)
	}
	if err != nil {
		return nil, err
	}
	return userReconstruct(self, sum)
}

func userListAdd(args []objects.Object) (objects.Object, error) {
	return userListConcat(args[0], args[1], true)
}

func userListRAdd(args []objects.Object) (objects.Object, error) {
	return userListConcat(args[0], args[1], false)
}

func userListIAdd(args []objects.Object) (objects.Object, error) {
	self, other := args[0], args[1]
	data, err := userData(self)
	if err != nil {
		return nil, err
	}
	rhs := other
	if isUser(other, userListClass) {
		if rhs, err = userData(other); err != nil {
			return nil, err
		}
	} else if other.TypeName() != "list" {
		elts, err := materialize(other)
		if err != nil {
			return nil, err
		}
		rhs = objects.NewList(elts)
	}
	if _, err := objects.CallMethod(data, "extend", []objects.Object{rhs}); err != nil {
		return nil, err
	}
	return self, nil
}

func userListMul(args []objects.Object) (objects.Object, error) {
	data, err := userData(args[0])
	if err != nil {
		return nil, err
	}
	prod, err := objects.Mul(data, args[1])
	if err != nil {
		return nil, err
	}
	return userReconstruct(args[0], prod)
}

func userListIMul(args []objects.Object) (objects.Object, error) {
	self := args[0]
	data, err := userData(self)
	if err != nil {
		return nil, err
	}
	prod, err := objects.Mul(data, args[1])
	if err != nil {
		return nil, err
	}
	if err := userSetData(self, prod); err != nil {
		return nil, err
	}
	return self, nil
}

// userListForward runs a mutating list method on self.data and returns None, the
// shape append/insert/remove/reverse/clear share.
func userListForward(self objects.Object, name string, rest []objects.Object) (objects.Object, error) {
	data, err := userData(self)
	if err != nil {
		return nil, err
	}
	if _, err := objects.CallMethod(data, name, rest); err != nil {
		return nil, err
	}
	return objects.None, nil
}

func userListAppend(args []objects.Object) (objects.Object, error) {
	return userListForward(args[0], "append", args[1:])
}

func userListInsert(args []objects.Object) (objects.Object, error) {
	return userListForward(args[0], "insert", args[1:])
}

func userListRemove(args []objects.Object) (objects.Object, error) {
	return userListForward(args[0], "remove", args[1:])
}

func userListReverse(args []objects.Object) (objects.Object, error) {
	return userListForward(args[0], "reverse", nil)
}

func userListClear(args []objects.Object) (objects.Object, error) {
	return userListForward(args[0], "clear", nil)
}

func userListExtend(args []objects.Object) (objects.Object, error) {
	other := args[1]
	if isUser(other, userListClass) {
		d, err := userData(other)
		if err != nil {
			return nil, err
		}
		other = d
	}
	return userListForward(args[0], "extend", []objects.Object{other})
}

func userListPop(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	data, err := userData(pos[0])
	if err != nil {
		return nil, err
	}
	return objects.CallMethod(data, "pop", pos[1:])
}

func userListCount(args []objects.Object) (objects.Object, error) {
	data, err := userData(args[0])
	if err != nil {
		return nil, err
	}
	return objects.CallMethod(data, "count", args[1:])
}

func userListIndex(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	data, err := userData(pos[0])
	if err != nil {
		return nil, err
	}
	return objects.CallMethod(data, "index", pos[1:])
}

func userListSort(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	data, err := userData(pos[0])
	if err != nil {
		return nil, err
	}
	if _, err := objects.CallMethodKw(data, "sort", pos[1:], kwNames, kwVals); err != nil {
		return nil, err
	}
	return objects.None, nil
}

func userListCopy(args []objects.Object) (objects.Object, error) {
	return userReconstruct(args[0], args[0])
}
