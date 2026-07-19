package objects

// A mapping proxy is the read-only face of a mapping. types.MappingProxyType(d)
// wraps a dict in one, and enum hands it out as a class's __members__ so callers
// can read the member table without being able to rebind it. Every read
// delegates to the wrapped dict, so the proxy stays a live view rather than a
// snapshot; every write raises the TypeError CPython gives, since the proxy has
// no mutating surface of its own.
type mappingProxyObject struct {
	d *dictObject
}

func (*mappingProxyObject) TypeName() string { return "mappingproxy" }

// NewMappingProxy wraps a mapping in a read-only proxy. CPython accepts a dict
// or another mapping and rejects a list or any non-mapping with a TypeError
// naming the offending type; the floor only ever proxies a dict, so that is the
// mapping this recognizes, unwrapping a proxy of a proxy to the same dict.
func NewMappingProxy(m Object) (Object, error) {
	switch x := m.(type) {
	case *dictObject:
		return &mappingProxyObject{d: x}, nil
	case *mappingProxyObject:
		return &mappingProxyObject{d: x.d}, nil
	}
	return nil, Raise(TypeError, "mappingproxy() argument must be a mapping, not %s", m.TypeName())
}

// mappingProxyMethod is the proxy's method surface: the read-only slice of the
// dict's methods. get, keys, values, items and copy delegate to the wrapped
// dict, and copy returns a real dict the way CPython's mappingproxy.copy does.
// A mutating name like pop or clear is not part of the surface, so it reads the
// same AttributeError any missing attribute would.
func mappingProxyMethod(p *mappingProxyObject, name string, args []Object) (Object, error) {
	switch name {
	case "get", "keys", "values", "items", "copy":
		return dictMethod(p.d, name, args)
	}
	return nil, noAttr(p, name)
}

// callTypeObject builds an instance of a constructor-less type value. Only
// mappingproxy can be built this way; every other type singleton is a kind the
// interpreter produces internally and exposes no Python constructor for, so
// calling it raises the "cannot create 'X' instances" TypeError CPython gives.
func callTypeObject(t *typeObject, args []Object) (Object, error) {
	if t.name == "mappingproxy" {
		switch len(args) {
		case 1:
			return NewMappingProxy(args[0])
		case 0:
			return nil, Raise(TypeError, "mappingproxy() missing required argument 'mapping' (pos 1)")
		default:
			return nil, Raise(TypeError, "mappingproxy() takes at most 1 argument (%d given)", len(args))
		}
	}
	if t.name == "types.GenericAlias" {
		// types.GenericAlias(origin, args) is the explicit constructor for what
		// origin[args] builds, so _collections_abc's classmethod(GenericAlias)
		// path reaches the same value as list[int].
		if len(args) != 2 {
			return nil, Raise(TypeError, "GenericAlias expected 2 arguments, got %d", len(args))
		}
		return NewGenericAlias(args[0], args[1]), nil
	}
	return nil, Raise(TypeError, "cannot create '%s' instances", t.name)
}
