package runtime

import "github.com/tamnd/unagi/pkg/objects"

// The four attribute builtins route to the same LoadAttrT/StoreAttrT/DelAttrT
// machinery the emitted x.attr code uses, carrying the calling thread so a
// threading.local reached through getattr/setattr/delattr sees the same private
// store the attribute syntax does. Each rejects a non-string name with the
// "attribute name must be string, not 'X'" TypeError CPython raises, and getattr
// and hasattr treat only AttributeError as the miss, letting any other exception
// propagate the way 3.14 does.

// isAttributeError reports whether err is an AttributeError, the only miss
// getattr and hasattr absorb.
func isAttributeError(err error) bool {
	e, ok := err.(*objects.Exception)
	return ok && e.Kind == objects.AttributeError
}

// attrName pulls the Go string out of a name argument, or reports the
// non-string TypeError against the offending value's type.
func attrName(name objects.Object) (string, error) {
	s, ok := objects.AsStr(name)
	if !ok {
		return "", objects.Raise(objects.TypeError, "attribute name must be string, not '%s'", name.TypeName())
	}
	return s, nil
}

// GetAttr is getattr(obj, name[, default]). With a default it swallows the
// AttributeError from a missing attribute and returns the default; any other
// error still propagates.
func GetAttr(t *objects.Thread, args []objects.Object) (objects.Object, error) {
	switch len(args) {
	case 2, 3:
	default:
		if len(args) < 2 {
			return nil, objects.Raise(objects.TypeError, "getattr expected at least 2 arguments, got %d", len(args))
		}
		return nil, objects.Raise(objects.TypeError, "getattr expected at most 3 arguments, got %d", len(args))
	}
	name, err := attrName(args[1])
	if err != nil {
		return nil, err
	}
	got, err := objects.LoadAttrT(t, args[0], name)
	if err != nil {
		if len(args) == 3 && isAttributeError(err) {
			return args[2], nil
		}
		return nil, err
	}
	return got, nil
}

// HasAttr is hasattr(obj, name): True unless the attribute read raises
// AttributeError, in which case False. A different error propagates.
func HasAttr(t *objects.Thread, args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "hasattr expected 2 arguments, got %d", len(args))
	}
	name, err := attrName(args[1])
	if err != nil {
		return nil, err
	}
	if _, err := objects.LoadAttrT(t, args[0], name); err != nil {
		if isAttributeError(err) {
			return objects.False, nil
		}
		return nil, err
	}
	return objects.True, nil
}

// SetAttr is setattr(obj, name, value).
func SetAttr(t *objects.Thread, args []objects.Object) (objects.Object, error) {
	if len(args) != 3 {
		return nil, objects.Raise(objects.TypeError, "setattr expected 3 arguments, got %d", len(args))
	}
	name, err := attrName(args[1])
	if err != nil {
		return nil, err
	}
	if err := objects.StoreAttrT(t, args[0], name, args[2]); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// DelAttr is delattr(obj, name).
func DelAttr(t *objects.Thread, args []objects.Object) (objects.Object, error) {
	if len(args) != 2 {
		return nil, objects.Raise(objects.TypeError, "delattr expected 2 arguments, got %d", len(args))
	}
	name, err := attrName(args[1])
	if err != nil {
		return nil, err
	}
	if err := objects.DelAttrT(t, args[0], name); err != nil {
		return nil, err
	}
	return objects.None, nil
}
