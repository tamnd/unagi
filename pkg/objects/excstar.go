package objects

// except* matching, PEP 654. SplitStar partitions an exception by whether
// each leaf matches the handler's types, preserving the nested group
// structure exactly like BaseExceptionGroup.split. CombineStar folds the
// unhandled remainder and the handler-raised exceptions back into the one
// exception that leaves the try. Both are pure tree logic on Exception, so
// they carry no runtime state and are unit tested directly. Every shape
// below was probed on 3.14.

// SplitStar partitions e by whether each leaf matches any of kinds. It
// returns the matched part and the unhandled remainder, either of which may
// be nil. A group keeps its message and structure in both halves, dropping
// only the branches that went the other way. A naked exception that matches
// comes back wrapped in an empty-message ExceptionGroup, which is what an
// except* clause binds; an unmatched naked exception stays naked in rest.
func SplitStar(e *Exception, kinds []string) (matched, rest *Exception) {
	match := func(x *Exception) bool {
		for _, k := range kinds {
			if Matches(x.Kind, k) {
				return true
			}
		}
		return false
	}
	var rec func(*Exception) (*Exception, *Exception)
	rec = func(x *Exception) (*Exception, *Exception) {
		if x.Group == nil {
			if match(x) {
				return x, nil
			}
			return nil, x
		}
		var m, u []*Exception
		for _, s := range x.Group {
			sm, su := rec(s)
			if sm != nil {
				m = append(m, sm)
			}
			if su != nil {
				u = append(u, su)
			}
		}
		var mg, ug *Exception
		if len(m) > 0 {
			mg = deriveGroup(x, m)
		}
		if len(u) > 0 {
			ug = deriveGroup(x, u)
		}
		return mg, ug
	}
	if e.Group == nil {
		if match(e) {
			return WrapGroup(e), nil
		}
		return nil, e
	}
	return rec(e)
}

// WrapGroup wraps a naked exception in an empty-message ExceptionGroup, the
// form except* binds when the raised exception is not already a group.
// Probed on 3.14: except* ValueError as e over raise ValueError binds e to
// an ExceptionGroup with an empty message wrapping the ValueError.
func WrapGroup(e *Exception) *Exception {
	return &Exception{
		Kind:  "ExceptionGroup",
		Args:  []Object{NewStr(""), NewTuple([]Object{e})},
		Group: []*Exception{e},
	}
}

// deriveGroup builds a new group with src's message and metadata but the
// given sub-exceptions, the operation BaseExceptionGroup.derive performs for
// each level of a split. The frames, cause, and context ride along so the
// remainder renders with the original raise site.
func deriveGroup(src *Exception, subs []*Exception) *Exception {
	objs := make([]Object, len(subs))
	for i, s := range subs {
		objs[i] = s
	}
	return &Exception{
		Kind:            src.Kind,
		Args:            []Object{src.Args[0], NewTuple(objs)},
		Group:           subs,
		Frames:          src.Frames,
		Cause:           src.Cause,
		Context:         src.Context,
		SuppressContext: src.SuppressContext,
		Notes:           src.Notes,
	}
}

// CombineStar folds the exceptions raised by except* handlers back together
// with the unhandled remainder into the exception that leaves the try, nil
// when everything was handled and nothing re-raised. Probed on 3.14: the
// remainder alone propagates as itself, a single raised exception with no
// remainder propagates bare, and any other mix wraps into an empty-message
// ExceptionGroup with the raised exceptions first and the remainder last.
func CombineStar(rest *Exception, raised []*Exception) *Exception {
	if len(raised) == 0 {
		return rest
	}
	all := append([]*Exception(nil), raised...)
	if rest != nil {
		all = append(all, rest)
	}
	if len(all) == 1 {
		return all[0]
	}
	objs := make([]Object, len(all))
	for i, s := range all {
		objs[i] = s
	}
	return &Exception{
		Kind:  "ExceptionGroup",
		Args:  []Object{NewStr(""), NewTuple(objs)},
		Group: all,
	}
}
