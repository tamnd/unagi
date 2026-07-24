package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// marshal is a built-in module. CPython implements it in C and importlib's
// _bootstrap_external imports it at module load to read and write .pyc code
// objects. unagi never produces code objects, so that path is dead here, but
// `import importlib` still needs marshal to resolve. This slice provides the
// documented data surface (dump/dumps/load/loads over the round-trippable value
// types) so both the import chain and a direct marshal caller work; the format
// itself lives in objects.MarshalDumps/MarshalLoads.

func init() {
	moduleTable["marshal"] = &moduleEntry{builtin: true, exec: initMarshal}
}

func initMarshal(m *objects.Module) error {
	for _, e := range []struct {
		name string
		obj  objects.Object
	}{
		{"version", objects.NewInt(objects.MarshalVersion)},
		{"dumps", objects.NewFuncKw("dumps", marshalDumps)},
		{"loads", objects.NewFuncKw("loads", marshalLoads)},
		{"dump", objects.NewFuncKw("dump", marshalDump)},
		{"load", objects.NewFuncKw("load", marshalLoad)},
	} {
		if err := objects.StoreAttr(m, e.name, e.obj); err != nil {
			return err
		}
	}
	return nil
}

// marshalDumps is marshal.dumps(value, version=version). The version argument is
// accepted for signature compatibility; unagi emits one format regardless, since
// the older on-disk formats only matter to a CPython that reads .pyc files.
func marshalDumps(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 1 {
		return nil, objects.Raise(objects.TypeError, "dumps() missing required argument 'value' (pos 1)")
	}
	if len(pos) > 2 {
		return nil, objects.Raise(objects.TypeError, "dumps() takes at most 2 arguments (%d given)", len(pos))
	}
	for _, name := range kwNames {
		if name != "version" {
			return nil, objects.Raise(objects.TypeError, "dumps() got an unexpected keyword argument '%s'", name)
		}
	}
	data, err := objects.MarshalDumps(pos[0])
	if err != nil {
		return nil, err
	}
	return objects.NewBytes(data), nil
}

// marshalLoads is marshal.loads(bytes). It reconstructs the first object the
// stream encodes and ignores any trailing bytes, matching CPython.
func marshalLoads(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) != 1 {
		return nil, objects.Raise(objects.TypeError, "loads() takes exactly one argument (%d given)", len(pos))
	}
	if len(kwNames) > 0 {
		return nil, objects.Raise(objects.TypeError, "loads() takes no keyword arguments")
	}
	data, ok := objects.AsBytes(pos[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "a bytes-like object is required, not '%s'", pos[0].TypeName())
	}
	return objects.MarshalLoads(data)
}

// marshalDump is marshal.dump(value, file, version=version). It marshals value
// and hands the bytes to file.write, the way CPython's dump drives the file
// object's write method.
func marshalDump(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 2 {
		return nil, objects.Raise(objects.TypeError, "dump() missing required argument 'file' (pos 2)")
	}
	if len(pos) > 3 {
		return nil, objects.Raise(objects.TypeError, "dump() takes at most 3 arguments (%d given)", len(pos))
	}
	for _, name := range kwNames {
		if name != "version" {
			return nil, objects.Raise(objects.TypeError, "dump() got an unexpected keyword argument '%s'", name)
		}
	}
	data, err := objects.MarshalDumps(pos[0])
	if err != nil {
		return nil, err
	}
	write, err := objects.LoadAttr(pos[1], "write")
	if err != nil {
		return nil, err
	}
	if _, err := objects.Call(write, []objects.Object{objects.NewBytes(data)}); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// marshalLoad is marshal.load(file). CPython reads only as many bytes as the
// object needs; unagi reads the whole file and unmarshals from the front, which
// gives the same object because loads stops at the first complete value.
func marshalLoad(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) != 1 {
		return nil, objects.Raise(objects.TypeError, "load() takes exactly one argument (%d given)", len(pos))
	}
	read, err := objects.LoadAttr(pos[0], "read")
	if err != nil {
		return nil, err
	}
	buf, err := objects.Call(read, nil)
	if err != nil {
		return nil, err
	}
	data, ok := objects.AsBytes(buf)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "file.read() returned not bytes but '%s'", buf.TypeName())
	}
	return objects.MarshalLoads(data)
}
