package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// genericBaseClass builds the typing.Generic base class _typing exports. Generic
// is a real class so `class Box(Generic[T])`, `issubclass(cls, Generic)`, and
// `super(Generic, cls)` all behave, and its __class_getitem__ builds a
// typing._GenericAlias by delegating to typing.py's own _generic_class_getitem,
// so Box[int] is the same _GenericAlias CPython produces rather than a bare
// types.GenericAlias. Subscription resolves the hook through the MRO, so a
// subclass inherits it.
func genericBaseClass() (objects.Object, error) {
	cgi := objects.NewFunc("__class_getitem__", 2, genericClassGetitem)
	isub := objects.NewFuncKwT("__init_subclass__", genericInitSubclass)
	return objects.NewClass(
		"Generic", "typing.Generic",
		[]objects.Object{objects.ObjectType()},
		[]string{"__class_getitem__", "__init_subclass__"},
		[]objects.Object{objects.NewClassMethod(cgi), objects.NewClassMethod(isub)},
		nil, nil,
	)
}

// genericInitSubclass is Generic.__init_subclass__(cls, **kwargs). Like
// __class_getitem__ it defers to typing.py's own _generic_init_subclass, which
// collects the subclass's type parameters from its __orig_bases__ and sets
// cls.__parameters__, so a `class Box(Generic[T])` gets the same __parameters__
// tuple CPython assigns. pos[0] is the subclass; any remaining positional and
// keyword arguments are the class keyword arguments to forward.
func genericInitSubclass(t *objects.Thread, pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	typingMod, err := ImportModule("typing")
	if err != nil {
		return nil, err
	}
	fn, err := objects.LoadAttr(typingMod, "_generic_init_subclass")
	if err != nil {
		return nil, err
	}
	return objects.CallKwT(t, fn, pos, kwNames, kwVals)
}

// genericClassGetitem is Generic.__class_getitem__(cls, item). It defers to
// typing._generic_class_getitem, the pure-Python builder typing.py defines, so
// the result is a faithful typing._GenericAlias with the validation and
// parameter handling typing expects. cls is the subscripted class and item the
// argument or argument tuple.
func genericClassGetitem(args []objects.Object) (objects.Object, error) {
	cls, item := args[0], args[1]
	typingMod, err := ImportModule("typing")
	if err != nil {
		return nil, err
	}
	fn, err := objects.LoadAttr(typingMod, "_generic_class_getitem")
	if err != nil {
		return nil, err
	}
	return objects.Call(fn, []objects.Object{cls, item})
}
