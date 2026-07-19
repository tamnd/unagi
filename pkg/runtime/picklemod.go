package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// pickle is a built-in module. CPython implements it in the _pickle C
// accelerator with a pure-Python pickle.py fallback; the runtime provides the
// serialization surface in Go under the same import name. This slice exposes
// dumps/loads over the scalar leaves at the binary protocols, plus the protocol
// constants and the exception hierarchy a program catches by name. Containers,
// the object-reduction protocol, and the file-based Pickler/Unpickler classes
// land in later slices.

func init() {
	moduleTable["pickle"] = &moduleEntry{builtin: true, exec: initPickle}
	moduleTable["_pickle"] = &moduleEntry{builtin: true, exec: initPickle}
}

func initPickle(m *objects.Module) error {
	for _, e := range []struct {
		name string
		obj  objects.Object
	}{
		{"dumps", objects.NewFuncKw("dumps", pickleDumps)},
		{"loads", objects.NewFuncKw("loads", pickleLoads)},
		{"DEFAULT_PROTOCOL", objects.NewInt(objects.PickleDefaultProtocol)},
		{"HIGHEST_PROTOCOL", objects.NewInt(objects.PickleHighestProtocol)},
		{"PickleError", objects.PickleErrorClass()},
		{"PicklingError", objects.PicklingErrorClass()},
		{"UnpicklingError", objects.UnpicklingErrorClass()},
	} {
		if err := objects.StoreAttr(m, e.name, e.obj); err != nil {
			return err
		}
	}
	return nil
}

// pickleDumps is pickle.dumps(obj, protocol=None, *, fix_imports=True,
// buffer_callback=None). It resolves the protocol the way CPython does — None
// means DEFAULT_PROTOCOL, a negative value means HIGHEST_PROTOCOL — and
// serializes obj. This slice supports the binary protocols 2..5; the text
// protocols 0 and 1 arrive in a later slice.
func pickleDumps(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 1 {
		return nil, objects.Raise(objects.TypeError, "dumps() missing required argument 'obj' (pos 1)")
	}
	obj := pos[0]

	var protoArg objects.Object
	if len(pos) >= 2 {
		protoArg = pos[1]
	}
	if len(pos) > 2 {
		return nil, objects.Raise(objects.TypeError, "dumps() takes at most 2 positional arguments (%d given)", len(pos))
	}
	for i, name := range kwNames {
		switch name {
		case "protocol":
			protoArg = kwVals[i]
		case "fix_imports", "buffer_callback":
			// Accepted for signature compatibility; fix_imports only affects the
			// text protocols, and buffer_callback drives protocol-5 out-of-band
			// buffers, both of which land in later slices.
		default:
			return nil, objects.Raise(objects.TypeError, "dumps() got an unexpected keyword argument '%s'", name)
		}
	}

	proto, err := resolvePickleProtocol(protoArg)
	if err != nil {
		return nil, err
	}
	data, err := objects.PickleDumps(obj, proto)
	if err != nil {
		return nil, err
	}
	return objects.NewBytes(data), nil
}

// resolvePickleProtocol turns the protocol argument into a concrete version,
// matching CPython's clamping: None picks the default, a negative value picks
// the highest, and a value above the highest is an error. This slice only
// emits the binary protocols, so a request for 0 or 1 is refused rather than
// answered with wrong bytes.
func resolvePickleProtocol(arg objects.Object) (int, error) {
	if arg == nil || arg == objects.None {
		return objects.PickleDefaultProtocol, nil
	}
	n, ok := objects.AsBigInt(arg)
	if !ok {
		return 0, objects.Raise(objects.TypeError, "an integer is required")
	}
	if !n.IsInt64() {
		return 0, objects.Raise(objects.ValueError, "pickle protocol must be <= %d", objects.PickleHighestProtocol)
	}
	proto := n.Int64()
	if proto < 0 {
		return objects.PickleHighestProtocol, nil
	}
	if proto > objects.PickleHighestProtocol {
		return 0, objects.Raise(objects.ValueError, "pickle protocol must be <= %d", objects.PickleHighestProtocol)
	}
	if proto < 2 {
		return 0, objects.Raise("NotImplementedError", "pickle protocol %d is not supported yet; use protocol 2 or higher", proto)
	}
	return int(proto), nil
}

// pickleLoads is pickle.loads(data, /, *, fix_imports=True, encoding='ASCII',
// errors='strict', buffers=()). It reconstructs the object a pickle encodes.
func pickleLoads(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 1 {
		return nil, objects.Raise(objects.TypeError, "loads() missing required argument 'data' (pos 1)")
	}
	if len(pos) > 1 {
		return nil, objects.Raise(objects.TypeError, "loads() takes 1 positional argument but %d were given", len(pos))
	}
	for _, name := range kwNames {
		switch name {
		case "fix_imports", "encoding", "errors", "buffers":
			// Accepted for signature compatibility; these steer the text
			// protocols and protocol-5 out-of-band buffers handled in later slices.
		default:
			return nil, objects.Raise(objects.TypeError, "loads() got an unexpected keyword argument '%s'", name)
		}
	}
	data, ok := objects.AsBytes(pos[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "a bytes-like object is required, not '%s'", pos[0].TypeName())
	}
	return objects.PickleLoads(data)
}
