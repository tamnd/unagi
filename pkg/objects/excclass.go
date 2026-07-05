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
