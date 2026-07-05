package objects

// Built-in exception types are first-class, subclassable class objects, one per
// name in excBases, linked into the same classObject graph user classes use.
// Synthesizing them here lets `Exception` read as a value, serve as a base, and
// answer isinstance, issubclass, and type() the way CPython's built-in exception
// classes do. The string Kind a raised *Exception carries stays the identity the
// traceback and matcher paths key on; ExcClass bridges that name back to the
// class object when a value-level question is asked.

// excClasses holds one synthesized class per built-in exception name. It is
// keyed by the canonical name, so the two OSError aliases resolve through
// excCanon rather than getting their own objects.
var excClasses = map[string]*classObject{}

func init() {
	// Build each class only once all its bases exist, so c3Linearize reads a
	// finished MRO off every base. The hierarchy is finite and acyclic, so each
	// pass makes progress and every name lands. Build order does not affect the
	// result: a class's bases and MRO are fixed by excBases, not by when it runs.
	pending := make(map[string][]string, len(excBases))
	for name, bases := range excBases {
		pending[name] = bases
	}
	for len(pending) > 0 {
		progressed := false
		for name, bases := range pending {
			ready := true
			for _, b := range bases {
				if _, ok := excClasses[b]; !ok {
					ready = false
					break
				}
			}
			if !ready {
				continue
			}
			baseObjs := make([]Object, len(bases))
			for i, b := range bases {
				baseObjs[i] = excClasses[b]
			}
			c, err := newClassCore(nil, name, name, baseObjs, nil, nil, nil, nil)
			if err != nil {
				panic("unagi: building builtin exception class " + name + ": " + err.Error())
			}
			excClasses[name] = c.(*classObject)
			delete(pending, name)
			progressed = true
		}
		if !progressed {
			panic("unagi: builtin exception hierarchy has an unbuildable name")
		}
	}
}

// ExcClass returns the synthesized class object for a built-in exception name,
// resolving the EnvironmentError and IOError aliases to OSError so IOError is
// OSError holds. ok is false for a name that is not a built-in exception.
func ExcClass(name string) (*classObject, bool) {
	c, ok := excClasses[excCanon(name)]
	return c, ok
}

// ExcClassValue is ExcClass typed as an Object, the form the runtime registers
// as a builtin the same way it registers object.
func ExcClassValue(name string) (Object, bool) {
	c, ok := ExcClass(name)
	if !ok {
		return nil, false
	}
	return c, true
}

// isExcClass reports whether c is an exception class: itself BaseException or
// any class deriving from it. Both built-in exception classes and user
// subclasses of them qualify, which is what makes their instances raisable and
// their names usable as except matchers.
func isExcClass(c *classObject) bool {
	base, ok := excClasses["BaseException"]
	if !ok {
		return false
	}
	return c == base || hasInMRO(c, base)
}

// IsExcClassValue reports whether a value is a class deriving from
// BaseException, the check an except matcher must pass. A matcher that fails it
// is the "catching classes that do not inherit from BaseException" TypeError.
func IsExcClassValue(o Object) bool {
	c, ok := o.(*classObject)
	return ok && isExcClass(c)
}

// AsRaisable converts a raised value to the exception it raises. An exception
// object raises itself; a bare exception class instantiates with no arguments
// the way `raise ValueError` does. ok is false for anything that cannot be
// raised, which the caller turns into CPython's derive-from-BaseException
// TypeError.
func AsRaisable(o Object) (*Exception, bool) {
	if e, ok := o.(*Exception); ok {
		return e, true
	}
	if c, ok := o.(*classObject); ok && isExcClass(c) {
		inst, err := Instantiate(c, nil, nil, nil)
		if err != nil {
			return nil, false
		}
		if e, ok := inst.(*Exception); ok {
			return e, true
		}
	}
	return nil, false
}

// ExcMatchesClass reports whether a raised exception is caught by one except
// matcher, given as a class value. A non-class matcher, or one that is not an
// exception class, matches nothing here; the mismatched-matcher TypeError is a
// later slice. Matching walks the exception's class MRO, so a user subclass is
// caught by any built-in or user base it derives from.
func ExcMatchesClass(e *Exception, cls Object) bool {
	c, ok := cls.(*classObject)
	if !ok || !isExcClass(c) {
		return false
	}
	ec, ok := excClassOf(e)
	if !ok {
		return false
	}
	if ec == c {
		return true
	}
	return hasInMRO(ec, c)
}

// ExcClassNames lists every built-in exception name that reads as a value,
// including the two OSError aliases, so the runtime can register them all.
func ExcClassNames() []string {
	names := make([]string, 0, len(excBases)+len(excAlias))
	for name := range excBases {
		names = append(names, name)
	}
	for name := range excAlias {
		names = append(names, name)
	}
	return names
}
