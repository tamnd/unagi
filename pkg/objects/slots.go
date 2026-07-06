package objects

import "strings"

// __slots__ trades the per-instance dict for a fixed set of named slots. A
// class body binding __slots__ makes type.__new__ install one member
// descriptor per slot name and drop __dict__ from instances, so writing an
// unlisted attribute raises instead of landing in a dict. The boxed tier keeps
// slot values in a per-instance map rather than a fixed layout; the observable
// protocol, the descriptors, the missing-dict errors, the class-variable
// conflict, and the layout rules are the probed 3.14 behavior. The typed tier
// maps slots to struct fields (plan 06), for which this records the names.

// memberDescriptor is the data descriptor type.__new__ installs for each slot
// name: reading it fetches the instance's slot value or raises, writing and
// deleting always intercept before any instance dict.
type memberDescriptor struct {
	name  string
	owner *classObject
}

func (*memberDescriptor) TypeName() string { return "member_descriptor" }

// applySlots processes a __slots__ binding in the class body: validate the
// names, mangle private ones the way the compiler mangles identifiers, reject
// a name the body also binds, and install a member descriptor per slot. The
// __dict__ and __weakref__ pseudo-slots install nothing; each re-adds what
// __slots__ removed and is disallowed when a base already provides it. The
// class records whether it declared slots and which names, and the caller
// derives the instance layout flags from that.
func applySlots(c *classObject) error {
	v, ok := c.dict["__slots__"]
	if !ok {
		return nil
	}
	c.hasSlots = true
	var items []Object
	if s, ok := v.(*strObject); ok {
		// A bare string is a single slot name, not an iterable of characters.
		items = []Object{s}
	} else {
		var err error
		items, err = iterAll(v)
		if err != nil {
			return err
		}
	}
	for _, it := range items {
		s, ok := it.(*strObject)
		if !ok {
			return Raise(TypeError, "__slots__ items must be strings, not '%s'", it.TypeName())
		}
		switch s.v {
		case "__dict__":
			if c.slotsDict || anyBase(c, func(b *classObject) bool { return b.instDict }) {
				return Raise(TypeError, "__dict__ slot disallowed: we already got one")
			}
			c.slotsDict = true
			continue
		case "__weakref__":
			if c.slotsWeakref || anyBase(c, func(b *classObject) bool { return b.instWeakref }) {
				return Raise(TypeError, "__weakref__ slot disallowed: we already got one")
			}
			c.slotsWeakref = true
			continue
		}
		name := mangleSlot(c.name, s.v)
		if _, dup := c.dict[name]; dup {
			if _, isSlot := c.dict[name].(*memberDescriptor); isSlot {
				// __slots__ = ('q', 'q') installs one descriptor and no error.
				continue
			}
			return Raise(ValueError, "'%s' in __slots__ conflicts with class variable", name)
		}
		c.dict[name] = &memberDescriptor{name: name, owner: c}
		c.order = append(c.order, name)
		c.slotNames = append(c.slotNames, name)
	}
	return nil
}

// mangleSlot applies private-name mangling to a slot name, the same rewrite
// the compiler applies to __name identifiers in a class body: two leading
// underscores and at most one trailing gain the class name, itself stripped
// of leading underscores.
func mangleSlot(clsName, slot string) string {
	if strings.HasPrefix(slot, "__") && !strings.HasSuffix(slot, "__") {
		return "_" + strings.TrimLeft(clsName, "_") + slot
	}
	return slot
}

// anyBase reports whether pred holds for any direct base.
func anyBase(c *classObject, pred func(*classObject) bool) bool {
	for _, b := range c.bases {
		if pred(b) {
			return true
		}
	}
	return false
}

// checkLayout enforces the instance lay-out rule on the direct bases: each
// base's solid base (its nearest ancestor adding slots) must sit on a single
// inheritance line, or the class would need two incompatible layouts. The
// probed wording matches type.__new__'s best_base failure.
func checkLayout(c *classObject) error {
	var winner *classObject
	for _, b := range c.bases {
		sb := solidBase(b)
		if sb == nil {
			continue
		}
		switch {
		case winner == nil, metaIsSubclass(sb, winner):
			winner = sb
		case metaIsSubclass(winner, sb):
		default:
			return Raise(TypeError, "multiple bases have instance lay-out conflict")
		}
	}
	return nil
}

// solidBase walks to the nearest class in the primary-base chain whose slots
// extend the instance layout; nil is the object root, which every layout
// extends.
func solidBase(c *classObject) *classObject {
	for c != nil {
		if len(c.slotNames) > 0 {
			return c
		}
		if len(c.bases) == 0 {
			return nil
		}
		c = c.bases[0]
	}
	return nil
}

// slotGet reads a slot through its member descriptor; an unset slot raises
// the plain no-attribute error.
func slotGet(x *instanceObject, d *memberDescriptor) (Object, error) {
	if v, ok := x.slots[d.name]; ok {
		return v, nil
	}
	return nil, Raise(AttributeError, "'%s' object has no attribute '%s'", x.cls.name, d.name)
}

// slotSet writes a slot value, allocating the per-instance store on first use.
func slotSet(x *instanceObject, d *memberDescriptor, val Object) error {
	if x.slots == nil {
		x.slots = map[string]Object{}
	}
	x.slots[d.name] = val
	return nil
}

// slotDel clears a slot; deleting an unset slot raises an AttributeError
// carrying just the name, the member descriptor's own wording.
func slotDel(x *instanceObject, d *memberDescriptor) error {
	if _, ok := x.slots[d.name]; !ok {
		return Raise(AttributeError, "%s", d.name)
	}
	delete(x.slots, d.name)
	return nil
}

// noDictSetError spells the two write failures on a dict-less instance: a
// class attribute that is not a data descriptor cannot be shadowed, and an
// unknown name has nowhere to land.
func noDictSetError(x *instanceObject, name string, classHasAttr bool) error {
	if classHasAttr {
		return Raise(AttributeError, "'%s' object attribute '%s' is read-only", x.cls.name, name)
	}
	return Raise(AttributeError,
		"'%s' object has no attribute '%s' and no __dict__ for setting new attributes", x.cls.name, name)
}
