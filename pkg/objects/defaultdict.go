package objects

import "strings"

// defaultdict is a dict subclass in CPython's _collections: an ordinary dict
// that carries a default_factory and overrides __missing__, so a subscript on
// an absent key calls the factory, stores the result under the key, and returns
// it. Because it is a dict subclass, unagi models it as a dictObject with the
// defaultDict kind rather than a separate type, so it shares the dict storage,
// methods, equality (a defaultdict equals a plain dict with the same items), and
// hashing behavior for free.

// NewDefaultDict builds a defaultdict with the given factory over the initial
// keys and values. A None factory is allowed and disables the missing-key fill.
func NewDefaultDict(factory Object, keys, vals []Object) (Object, error) {
	d, err := NewDict(keys, vals)
	if err != nil {
		return nil, err
	}
	dd := d.(*dictObject)
	dd.kind = defaultDict
	dd.factory = factory
	return dd, nil
}

// dictSubscript reads d[key], routing a defaultdict's missing key through its
// factory the way __missing__ does. A plain dict, or a defaultdict whose factory
// is None, raises the ordinary KeyError.
func dictSubscript(d *dictObject, key Object) (Object, error) {
	if d.kind == counterDict {
		// Counter.__missing__ returns a zero count without storing the key, so a
		// read never grows the mapping.
		if v, ok, err := d.lookup(key); err != nil {
			return nil, err
		} else if ok {
			return v, nil
		}
		return NewInt(0), nil
	}
	if d.kind != defaultDict || d.factory == nil || d.factory == None {
		return d.get(key)
	}
	if v, ok, err := d.lookup(key); err != nil {
		return nil, err
	} else if ok {
		return v, nil
	}
	// The factory is called with no arguments; whatever it returns is stored
	// under the key and handed back, matching defaultdict.__missing__.
	v, err := Call(d.factory, nil)
	if err != nil {
		return nil, err
	}
	if err := d.set(key, v); err != nil {
		return nil, err
	}
	return v, nil
}

// dictDefaultFactory reads the default_factory attribute: the stored factory, or
// None when the defaultdict was built without one.
func dictDefaultFactory(d *dictObject) Object {
	if d.factory == nil {
		return None
	}
	return d.factory
}

// defaultDictRepr spells defaultdict(<factory>, <dict>), the factory repr
// followed by the ordinary dict body, matching CPython.
func defaultDictRepr(d *dictObject, strict bool) (string, error) {
	factory, err := reprCore(dictDefaultFactory(d), strict)
	if err != nil {
		return "", err
	}
	body, err := dictBodyRepr(d, strict)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("defaultdict(")
	b.WriteString(factory)
	b.WriteString(", ")
	b.WriteString(body)
	b.WriteString(")")
	return b.String(), nil
}
