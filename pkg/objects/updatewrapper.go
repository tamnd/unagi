package objects

// This file holds the body of functools.update_wrapper. It makes a wrapper
// function look like the function it wraps by copying a fixed set of attributes,
// merging the wrapped __dict__ into the wrapper's, and recording the original on
// wrapper.__wrapped__. The function attribute protocol in funcattrs.go carries
// the writable slots and dict this relies on.

// WrapperAssignments and WrapperUpdates are the functools defaults: the
// attributes update_wrapper copies straight across and the dict attributes it
// merges. They match CPython's WRAPPER_ASSIGNMENTS and WRAPPER_UPDATES.
var (
	WrapperAssignments = []string{"__module__", "__name__", "__qualname__", "__annotations__", "__doc__"}
	WrapperUpdates     = []string{"__dict__"}
)

// UpdateWrapper copies each attribute named in assigned from wrapped onto
// wrapper, merges each dict named in updated, and sets wrapper.__wrapped__ to
// wrapped, returning wrapper. A wrapped object that lacks an assigned attribute
// is skipped and a missing updated dict contributes nothing, the getattr with
// default and setattr shape CPython's update_wrapper performs. assigned and
// updated are iterables of attribute names, so a caller can override the
// defaults the way functools does.
func UpdateWrapper(wrapper, wrapped, assigned, updated Object) (Object, error) {
	names, err := nameList(assigned)
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		val, err := LoadAttr(wrapped, name)
		if err != nil {
			if isAttrError(err) {
				continue
			}
			return nil, err
		}
		if err := StoreAttr(wrapper, name, val); err != nil {
			return nil, err
		}
	}

	upd, err := nameList(updated)
	if err != nil {
		return nil, err
	}
	for _, name := range upd {
		target, err := LoadAttr(wrapper, name)
		if err != nil {
			return nil, err
		}
		td, ok := target.(*dictObject)
		if !ok {
			return nil, Raise(AttributeError,
				"'%s' object has no attribute 'update'", target.TypeName())
		}
		src, err := LoadAttr(wrapped, name)
		if err != nil {
			if isAttrError(err) {
				continue
			}
			return nil, err
		}
		if err := td.mergeMapping(src); err != nil {
			return nil, err
		}
	}

	if err := StoreAttr(wrapper, "__wrapped__", wrapped); err != nil {
		return nil, err
	}
	return wrapper, nil
}

// nameList reads an assigned or updated argument, an iterable of attribute
// names, into a slice of Go strings.
func nameList(v Object) ([]string, error) {
	it, err := Iter(v)
	if err != nil {
		return nil, err
	}
	var names []string
	for {
		e, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return names, nil
		}
		names = append(names, Str(e))
	}
}
