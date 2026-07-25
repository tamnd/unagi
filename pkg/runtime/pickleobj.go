package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// The file-based Pickler and Unpickler classes, plus the dump/load module
// functions and the bytes_types tuple. CPython implements these in the _pickle
// accelerator (with a pure pickle.py fallback); the runtime provides them in Go
// over the same PickleDumps/PickleLoads engine dumps/loads already use, so a
// Pickler writes the whole pickle to its file in one write and an Unpickler
// reads the whole file back and reconstructs from it. This is what shelve needs:
// Pickler(f, protocol).dump(value) and Unpickler(f).load().

const (
	picklerFileAttr  = "_unagi_pickle_file"
	picklerProtoAttr = "_unagi_pickle_proto"
)

// registerPickleObjects installs Pickler, Unpickler, dump, load and bytes_types
// on the pickle module.
func registerPickleObjects(m *objects.Module) error {
	picklerCls, err := buildPicklerClass()
	if err != nil {
		return err
	}
	unpicklerCls, err := buildUnpicklerClass()
	if err != nil {
		return err
	}
	// bytes_types is the pair pickletools and pickle.py compare against; it is
	// exactly (bytes, bytearray) in CPython 3.14. BuiltinFn resolves the builtin
	// name to its type object, the same value the bare `bytes` name evaluates to.
	bytesTypes := objects.NewTuple([]objects.Object{BuiltinFn("bytes"), BuiltinFn("bytearray")})

	for _, e := range []struct {
		name string
		obj  objects.Object
	}{
		{"Pickler", picklerCls},
		{"Unpickler", unpicklerCls},
		{"dump", objects.NewFuncKw("dump", pickleDump)},
		{"load", objects.NewFuncKw("load", pickleLoad)},
		{"bytes_types", bytesTypes},
	} {
		if err := objects.StoreAttr(m, e.name, e.obj); err != nil {
			return err
		}
	}
	return nil
}

func buildPicklerClass() (objects.Object, error) {
	names := []string{"__init__", "dump", "clear_memo"}
	vals := []objects.Object{
		objects.NewMethodKw("__init__", picklerInit),
		objects.NewMethod("dump", 2, picklerDump),
		objects.NewMethod("clear_memo", 1, picklerClearMemo),
	}
	return objects.NewClass("Pickler", "pickle.Pickler", nil, names, vals, nil, nil)
}

func buildUnpicklerClass() (objects.Object, error) {
	names := []string{"__init__", "load"}
	vals := []objects.Object{
		objects.NewMethodKw("__init__", unpicklerInit),
		objects.NewMethod("load", 1, unpicklerLoad),
	}
	return objects.NewClass("Unpickler", "pickle.Unpickler", nil, names, vals, nil, nil)
}

// picklerInit is Pickler(file, protocol=None, *, fix_imports=True,
// buffer_callback=None). It records the file and the resolved protocol; the file
// must have a write attribute, the way CPython requires.
func picklerInit(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 2 {
		return nil, objects.Raise(objects.TypeError, "Pickler() missing required argument 'file' (pos 1)")
	}
	self, file := pos[0], pos[1]
	var protoArg objects.Object
	if len(pos) >= 3 {
		protoArg = pos[2]
	}
	if len(pos) > 3 {
		return nil, objects.Raise(objects.TypeError, "Pickler() takes at most 2 positional arguments (%d given)", len(pos)-1)
	}
	for i, name := range kwNames {
		switch name {
		case "protocol":
			protoArg = kwVals[i]
		case "fix_imports", "buffer_callback":
			// Accepted for signature compatibility; they steer the text protocols
			// and protocol-5 out-of-band buffers the engine handles elsewhere.
		default:
			return nil, objects.Raise(objects.TypeError, "Pickler() got an unexpected keyword argument '%s'", name)
		}
	}
	if !ioHasAttr(file, "write") {
		return nil, objects.Raise(objects.TypeError, "file must have a 'write' attribute")
	}
	proto, err := resolvePickleProtocol(protoArg)
	if err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, picklerFileAttr, file); err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(self, picklerProtoAttr, objects.NewInt(int64(proto))); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// picklerDump is Pickler.dump(obj): it serializes obj and writes the whole
// pickle to the file in one write, then returns None as CPython does.
func picklerDump(args []objects.Object) (objects.Object, error) {
	self, obj := args[0], args[1]
	file, err := objects.LoadAttr(self, picklerFileAttr)
	if err != nil {
		return nil, objects.Raise(objects.TypeError, "Pickler.dump() called on an uninitialized Pickler")
	}
	protoObj, err := objects.LoadAttr(self, picklerProtoAttr)
	if err != nil {
		return nil, objects.Raise(objects.TypeError, "Pickler.dump() called on an uninitialized Pickler")
	}
	proto, _ := objects.AsInt(protoObj)
	data, err := objects.PickleDumps(obj, int(proto))
	if err != nil {
		return nil, err
	}
	if _, err := objects.CallMethod(file, "write", []objects.Object{objects.NewBytes(data)}); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// picklerClearMemo is Pickler.clear_memo(). This runtime holds no cross-dump memo
// (each dump serializes standalone), so it is a no-op, matching the observable
// effect of clearing an empty memo.
func picklerClearMemo(args []objects.Object) (objects.Object, error) {
	return objects.None, nil
}

// unpicklerInit is Unpickler(file, *, fix_imports=True, encoding='ASCII',
// errors='strict', buffers=()). The file must have a read attribute.
func unpicklerInit(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 2 {
		return nil, objects.Raise(objects.TypeError, "Unpickler() missing required argument 'file' (pos 1)")
	}
	self, file := pos[0], pos[1]
	if len(pos) > 2 {
		return nil, objects.Raise(objects.TypeError, "Unpickler() takes 1 positional argument but %d were given", len(pos)-1)
	}
	for _, name := range kwNames {
		switch name {
		case "fix_imports", "encoding", "errors", "buffers":
			// Accepted for signature compatibility; they steer the text protocols
			// and protocol-5 out-of-band buffers handled by the engine.
		default:
			return nil, objects.Raise(objects.TypeError, "Unpickler() got an unexpected keyword argument '%s'", name)
		}
	}
	if !ioHasAttr(file, "read") {
		return nil, objects.Raise(objects.TypeError, "file must have 'read' and 'readline' attributes")
	}
	if err := objects.StoreAttr(self, picklerFileAttr, file); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// unpicklerLoad is Unpickler.load(): it reads the whole file and reconstructs the
// object the pickle encodes.
func unpicklerLoad(args []objects.Object) (objects.Object, error) {
	self := args[0]
	file, err := objects.LoadAttr(self, picklerFileAttr)
	if err != nil {
		return nil, objects.Raise(objects.TypeError, "Unpickler.load() called on an uninitialized Unpickler")
	}
	r, err := objects.CallMethod(file, "read", nil)
	if err != nil {
		return nil, err
	}
	data, ok := objects.AsBytes(r)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "read() must return bytes, not '%s'", r.TypeName())
	}
	return objects.PickleLoads(data)
}

// pickleDump is the module function pickle.dump(obj, file, protocol=None, *,
// fix_imports=True, buffer_callback=None) — Pickler(file, protocol).dump(obj).
func pickleDump(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 2 {
		return nil, objects.Raise(objects.TypeError, "dump() missing required argument 'file' (pos 2)")
	}
	obj, file := pos[0], pos[1]
	var protoArg objects.Object
	if len(pos) >= 3 {
		protoArg = pos[2]
	}
	if len(pos) > 3 {
		return nil, objects.Raise(objects.TypeError, "dump() takes at most 3 positional arguments (%d given)", len(pos))
	}
	for i, name := range kwNames {
		switch name {
		case "protocol":
			protoArg = kwVals[i]
		case "fix_imports", "buffer_callback":
		default:
			return nil, objects.Raise(objects.TypeError, "dump() got an unexpected keyword argument '%s'", name)
		}
	}
	if !ioHasAttr(file, "write") {
		return nil, objects.Raise(objects.TypeError, "file must have a 'write' attribute")
	}
	proto, err := resolvePickleProtocol(protoArg)
	if err != nil {
		return nil, err
	}
	data, err := objects.PickleDumps(obj, proto)
	if err != nil {
		return nil, err
	}
	if _, err := objects.CallMethod(file, "write", []objects.Object{objects.NewBytes(data)}); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// pickleLoad is the module function pickle.load(file, *, fix_imports=True,
// encoding='ASCII', errors='strict', buffers=()).
func pickleLoad(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 1 {
		return nil, objects.Raise(objects.TypeError, "load() missing required argument 'file' (pos 1)")
	}
	if len(pos) > 1 {
		return nil, objects.Raise(objects.TypeError, "load() takes 1 positional argument but %d were given", len(pos))
	}
	for _, name := range kwNames {
		switch name {
		case "fix_imports", "encoding", "errors", "buffers":
		default:
			return nil, objects.Raise(objects.TypeError, "load() got an unexpected keyword argument '%s'", name)
		}
	}
	file := pos[0]
	r, err := objects.CallMethod(file, "read", nil)
	if err != nil {
		return nil, err
	}
	data, ok := objects.AsBytes(r)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "read() must return bytes, not '%s'", r.TypeName())
	}
	return objects.PickleLoads(data)
}
