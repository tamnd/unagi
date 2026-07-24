package objects

// typeSlotDescriptor is a getset descriptor CPython installs in type.__dict__,
// such as type.__dict__['__mro__'] and type.__dict__['__dict__']. inspect binds
// their __get__ once (`_static_getmro = type.__dict__['__mro__'].__get__`,
// `_get_dunder_dict_of_class = type.__dict__['__dict__'].__get__`) and calls it
// to read a class attribute without tripping a metaclass __getattribute__, so
// the descriptor only has to answer __get__ with a reader that returns the
// owner's value for the named slot.
type typeSlotDescriptor struct{ attr string }

func (*typeSlotDescriptor) TypeName() string { return "getset_descriptor" }

// The shared descriptor instances stored in type.__dict__.
var (
	typeMroDescriptor  = &typeSlotDescriptor{attr: "__mro__"}
	typeDictDescriptor = &typeSlotDescriptor{attr: "__dict__"}
)

// get is the descriptor's __get__: it hands back the owner class's value for the
// slot. The class flows in as the first argument (the instance the descriptor is
// read against), and every class type unagi has, a user class or a builtin type
// object, already answers these slots through LoadAttr, so the reader reuses
// that path. The owner argument is accepted and ignored, matching __get__'s
// (instance, owner=None) shape.
func (d *typeSlotDescriptor) get(args []Object) (Object, error) {
	if len(args) < 1 {
		return nil, Raise(TypeError, "__get__() missing required argument")
	}
	return LoadAttr(args[0], d.attr)
}
