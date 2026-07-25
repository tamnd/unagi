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
		{"encode_long", objects.NewFunc("encode_long", 1, pickleEncodeLong)},
		{"decode_long", objects.NewFunc("decode_long", 1, pickleDecodeLong)},
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
	// The opcode byte constants and __all__. pickletools imports fine without
	// them until its module-body assure_pickle_consistency() cross-checks its own
	// opcode table against pickle.__all__, reading each uppercase name off the
	// module and demanding an exact bijection of one-byte codes. CPython builds
	// these in pickle.py; the Go shim shadows that file, so it carries the same
	// names and byte values here.
	if err := registerPickleOpcodes(m); err != nil {
		return err
	}
	// The file-based Pickler/Unpickler classes, the dump/load module functions
	// and bytes_types, over the same engine dumps/loads use.
	return registerPickleObjects(m)
}

// registerPickleOpcodes stores the pickle opcode byte constants and __all__ on
// the module. The one-byte opcodes are the alphabet pickletools disassembles and
// cross-checks; FALSE and TRUE are the two multi-byte text-protocol codes the
// same file names. __all__ mirrors CPython's, the base names followed by the
// protocol constants and the uppercase opcode names dir() would surface.
func registerPickleOpcodes(m *objects.Module) error {
	opcodes := []struct {
		name string
		code []byte
	}{
		{"ADDITEMS", []byte{0x90}}, {"APPEND", []byte{0x61}}, {"APPENDS", []byte{0x65}},
		{"BINBYTES", []byte{0x42}}, {"BINBYTES8", []byte{0x8e}}, {"BINFLOAT", []byte{0x47}},
		{"BINGET", []byte{0x68}}, {"BININT", []byte{0x4a}}, {"BININT1", []byte{0x4b}},
		{"BININT2", []byte{0x4d}}, {"BINPERSID", []byte{0x51}}, {"BINPUT", []byte{0x71}},
		{"BINSTRING", []byte{0x54}}, {"BINUNICODE", []byte{0x58}}, {"BINUNICODE8", []byte{0x8d}},
		{"BUILD", []byte{0x62}}, {"BYTEARRAY8", []byte{0x96}}, {"DICT", []byte{0x64}},
		{"DUP", []byte{0x32}}, {"EMPTY_DICT", []byte{0x7d}}, {"EMPTY_LIST", []byte{0x5d}},
		{"EMPTY_SET", []byte{0x8f}}, {"EMPTY_TUPLE", []byte{0x29}}, {"EXT1", []byte{0x82}},
		{"EXT2", []byte{0x83}}, {"EXT4", []byte{0x84}}, {"FLOAT", []byte{0x46}},
		{"FRAME", []byte{0x95}}, {"FROZENSET", []byte{0x91}}, {"GET", []byte{0x67}},
		{"GLOBAL", []byte{0x63}}, {"INST", []byte{0x69}}, {"INT", []byte{0x49}},
		{"LIST", []byte{0x6c}}, {"LONG", []byte{0x4c}}, {"LONG1", []byte{0x8a}},
		{"LONG4", []byte{0x8b}}, {"LONG_BINGET", []byte{0x6a}}, {"LONG_BINPUT", []byte{0x72}},
		{"MARK", []byte{0x28}}, {"MEMOIZE", []byte{0x94}}, {"NEWFALSE", []byte{0x89}},
		{"NEWOBJ", []byte{0x81}}, {"NEWOBJ_EX", []byte{0x92}}, {"NEWTRUE", []byte{0x88}},
		{"NEXT_BUFFER", []byte{0x97}}, {"NONE", []byte{0x4e}}, {"OBJ", []byte{0x6f}},
		{"PERSID", []byte{0x50}}, {"POP", []byte{0x30}}, {"POP_MARK", []byte{0x31}},
		{"PROTO", []byte{0x80}}, {"PUT", []byte{0x70}}, {"READONLY_BUFFER", []byte{0x98}},
		{"REDUCE", []byte{0x52}}, {"SETITEM", []byte{0x73}}, {"SETITEMS", []byte{0x75}},
		{"SHORT_BINBYTES", []byte{0x43}}, {"SHORT_BINSTRING", []byte{0x55}}, {"SHORT_BINUNICODE", []byte{0x8c}},
		{"STACK_GLOBAL", []byte{0x93}}, {"STOP", []byte{0x2e}}, {"STRING", []byte{0x53}},
		{"TUPLE", []byte{0x74}}, {"TUPLE1", []byte{0x85}}, {"TUPLE2", []byte{0x86}},
		{"TUPLE3", []byte{0x87}}, {"UNICODE", []byte{0x56}},
		// FALSE and TRUE are the multi-byte text codes, `I00\n` and `I01\n`.
		{"FALSE", []byte{0x49, 0x30, 0x30, 0x0a}}, {"TRUE", []byte{0x49, 0x30, 0x31, 0x0a}},
	}
	for _, op := range opcodes {
		if err := objects.StoreAttr(m, op.name, objects.NewBytes(op.code)); err != nil {
			return err
		}
	}
	names := []string{
		"PickleError", "PicklingError", "UnpicklingError", "Pickler", "Unpickler",
		"dump", "dumps", "load", "loads", "PickleBuffer", "ADDITEMS", "APPEND",
		"APPENDS", "BINBYTES", "BINBYTES8", "BINFLOAT", "BINGET", "BININT", "BININT1",
		"BININT2", "BINPERSID", "BINPUT", "BINSTRING", "BINUNICODE", "BINUNICODE8",
		"BUILD", "BYTEARRAY8", "DEFAULT_PROTOCOL", "DICT", "DUP", "EMPTY_DICT",
		"EMPTY_LIST", "EMPTY_SET", "EMPTY_TUPLE", "EXT1", "EXT2", "EXT4", "FALSE",
		"FLOAT", "FRAME", "FROZENSET", "GET", "GLOBAL", "HIGHEST_PROTOCOL", "INST",
		"INT", "LIST", "LONG", "LONG1", "LONG4", "LONG_BINGET", "LONG_BINPUT", "MARK",
		"MEMOIZE", "NEWFALSE", "NEWOBJ", "NEWOBJ_EX", "NEWTRUE", "NEXT_BUFFER", "NONE",
		"OBJ", "PERSID", "POP", "POP_MARK", "PROTO", "PUT", "READONLY_BUFFER", "REDUCE",
		"SETITEM", "SETITEMS", "SHORT_BINBYTES", "SHORT_BINSTRING", "SHORT_BINUNICODE",
		"STACK_GLOBAL", "STOP", "STRING", "TRUE", "TUPLE", "TUPLE1", "TUPLE2", "TUPLE3",
		"UNICODE",
	}
	all := make([]objects.Object, len(names))
	for i, n := range names {
		all[i] = objects.NewStr(n)
	}
	return objects.StoreAttr(m, "__all__", objects.NewList(all))
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

// pickleEncodeLong is pickle.encode_long(x): the two's-complement
// little-endian bytes for an integer, empty for zero. pickletools pairs it with
// decode_long to render LONG opcodes.
func pickleEncodeLong(args []objects.Object) (objects.Object, error) {
	n, ok := objects.AsBigInt(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
	}
	return objects.NewBytes(objects.EncodeLong(n)), nil
}

// pickleDecodeLong is pickle.decode_long(data): the integer a two's-complement
// little-endian byte string denotes. pickletools imports this name directly.
func pickleDecodeLong(args []objects.Object) (objects.Object, error) {
	body, ok := objects.AsBufferBytes(args[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "argument should be a bytes-like object, not '%s'", args[0].TypeName())
	}
	return objects.NewIntFromBig(objects.DecodeLong(body)), nil
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
