package objects

import (
	"slices"
	"strings"
)

// unionObject is a PEP 604 type union, the X | Y form. Its members are the type
// objects that make it up, flattened and deduplicated in first-seen order the
// way CPython builds one. type(int | str) is typing.Union, the union hashes and
// compares as an unordered set of its members, __args__ hands the members back
// as a tuple, and isinstance walks them. Reading None as a member stores the
// NoneType type object but spells it None in the repr, matching int | None.
type unionObject struct {
	args []Object
}

func (*unionObject) TypeName() string { return "typing.Union" }

// asUnionMember reports whether o can be a member of a type union and returns
// the object stored for it. A class, a type value, and a builtin type
// constructor stand for themselves; None stands for its NoneType type object,
// so int | None keeps NoneType in __args__ the way CPython does. A plain builtin
// function like len is not a type, so it cannot join a union.
func asUnionMember(o Object) (Object, bool) {
	switch t := o.(type) {
	case *classObject:
		return t, true
	case *typeObject:
		return t, true
	case *genericAliasObject:
		// A parameterized generic like dict[str, str] stands for itself in a
		// union, so dict[str, str] | None keeps the alias in __args__ and reprs it
		// the way CPython does.
		return t, true
	case *funcObject:
		if builtinTypeReprs[t.name] {
			return t, true
		}
		return nil, false
	}
	if o == None {
		return TypeSingleton("NoneType"), true
	}
	return nil, false
}

// unionOr builds the union for a | b when either operand is already a union or a
// type. ok is false when neither side takes part, letting BitOr fall through to
// its numeric and set paths. A type on one side and a non-type on the other is
// the TypeError CPython raises for type.__or__; the permissive union.__or__ that
// accepts typing special forms is not modeled here.
func unionOr(a, b Object) (Object, bool, error) {
	au, aIsUnion := a.(*unionObject)
	bu, bIsUnion := b.(*unionObject)
	am, aIsType := asUnionMember(a)
	bm, bIsType := asUnionMember(b)
	if !aIsUnion && !bIsUnion && !aIsType && !bIsType {
		return nil, false, nil
	}
	var members []Object
	switch {
	case aIsUnion:
		members = append(members, au.args...)
	case aIsType:
		members = append(members, am)
	default:
		return nil, true, unionOperandErr(a, b)
	}
	switch {
	case bIsUnion:
		members = append(members, bu.args...)
	case bIsType:
		members = append(members, bm)
	default:
		return nil, true, unionOperandErr(a, b)
	}
	return buildUnion(members), true, nil
}

// buildUnion deduplicates the members by identity, keeping first-seen order, and
// collapses a single survivor back to that type: int | int is int, not a union,
// the same as CPython.
func buildUnion(members []Object) Object {
	uniq := make([]Object, 0, len(members))
	for _, m := range members {
		if !slices.Contains(uniq, m) {
			uniq = append(uniq, m)
		}
	}
	if len(uniq) == 1 {
		return uniq[0]
	}
	return &unionObject{args: uniq}
}

// unionOperandErr is the TypeError for mixing a type with a non-type under |,
// spelling the type side as 'type' and None as 'NoneType' the way CPython does.
func unionOperandErr(a, b Object) error {
	return Raise(TypeError, "unsupported operand type(s) for |: '%s' and '%s'",
		unionOperandLabel(a), unionOperandLabel(b))
}

func unionOperandLabel(o Object) string {
	if o == None {
		return "NoneType"
	}
	switch t := o.(type) {
	case *classObject, *typeObject:
		return "type"
	case *funcObject:
		if builtinTypeReprs[t.name] {
			return "type"
		}
	}
	return o.TypeName()
}

// unionArgsEqual reports whether two unions have the same set of members. The
// args are already deduplicated, so equal length plus a subset check is set
// equality, which makes int | str equal str | int.
func unionArgsEqual(x, y *unionObject) bool {
	if len(x.args) != len(y.args) {
		return false
	}
	for _, a := range x.args {
		if !slices.Contains(y.args, a) {
			return false
		}
	}
	return true
}

// pyHashUnion hashes a union so equal unions hash alike regardless of order: the
// member hashes fold together with xor, matching hash(int | str) == hash(str |
// int). The members hash by identity, so the value is stable within a run, which
// is all a dict key needs.
func pyHashUnion(u *unionObject) (int64, error) {
	var h uint64
	for _, m := range u.args {
		mh, err := PyHash(m)
		if err != nil {
			return 0, err
		}
		h ^= uint64(mh)
	}
	r := int64(h)
	if r == -1 {
		r = -2
	}
	return r, nil
}

// unionRepr renders the members joined by | with NoneType spelled None, so int |
// str and int | None read the way CPython prints them.
func unionRepr(u *unionObject) string {
	parts := make([]string, len(u.args))
	for i, m := range u.args {
		parts[i] = unionMemberRepr(m)
	}
	return strings.Join(parts, " | ")
}

func unionMemberRepr(m Object) string {
	switch t := m.(type) {
	case *typeObject:
		if t.name == "NoneType" {
			return "None"
		}
		return t.name
	case *funcObject:
		return t.name
	case *classObject:
		return t.qual
	case *genericAliasObject:
		// CPython renders a union member with _type_repr, so an alias member shows
		// as dict[str, str], not its type name.
		if s, err := genericAliasRepr(t); err == nil {
			return s
		}
	}
	return m.TypeName()
}

// unionLoadAttr answers the attributes a union exposes: __args__ is the member
// tuple pickle_union reads, and __parameters__ is empty because a concrete union
// carries no type variables.
func unionLoadAttr(u *unionObject, name string) (Object, error) {
	switch name {
	case "__args__":
		return NewTuple(append([]Object{}, u.args...)), nil
	case "__parameters__":
		return NewTuple(nil), nil
	}
	return nil, Raise(AttributeError, "'typing.Union' object has no attribute '%s'", name)
}
