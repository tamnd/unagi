package objects

import (
	"encoding/binary"
	"math"
	"math/big"
)

// PickleLoads reconstructs the object a pickle stream encodes. It is a stack
// machine: most opcodes push a value, memo opcodes store and fetch shared
// values, and STOP pops the final result. This slice reads the scalar and memo
// opcodes the scalar pickler emits; container and reduction opcodes are added
// alongside their pickler support in later slices. Byte-identity is a
// dump-side property, so the loader only needs to round-trip faithfully.
type unpickler struct {
	data  []byte
	pos   int
	stack []Object
	memo  []Object
	marks []int // stack positions a MARK opcode recorded, for variable builds
}

// PickleLoads parses a pickle and returns the top object.
func PickleLoads(data []byte) (Object, error) {
	u := &unpickler{data: data}
	for {
		op, err := u.readByte()
		if err != nil {
			return nil, newUnpicklingError("pickle data was truncated")
		}
		switch op {
		case opProto:
			if _, err := u.readByte(); err != nil {
				return nil, newUnpicklingError("pickle data was truncated")
			}
		case opFrame:
			// The frame length is advisory for an in-memory reader; skip it and
			// keep reading the enclosed opcodes inline.
			if _, err := u.readN(8); err != nil {
				return nil, newUnpicklingError("pickle data was truncated")
			}
		case opStop:
			if len(u.stack) == 0 {
				return nil, newUnpicklingError("unpickling stack underflow")
			}
			return u.stack[len(u.stack)-1], nil
		default:
			if err := u.dispatch(op); err != nil {
				return nil, err
			}
		}
	}
}

// dispatch handles one non-structural opcode.
func (u *unpickler) dispatch(op byte) error {
	switch op {
	case opNone:
		u.push(None)
	case opNewTrue:
		u.push(True)
	case opNewFalse:
		u.push(False)
	case opBinInt1:
		b, err := u.readByte()
		if err != nil {
			return u.truncated()
		}
		u.push(NewInt(int64(b)))
	case opBinInt2:
		b, err := u.readN(2)
		if err != nil {
			return u.truncated()
		}
		u.push(NewInt(int64(binary.LittleEndian.Uint16(b))))
	case opBinInt:
		b, err := u.readN(4)
		if err != nil {
			return u.truncated()
		}
		u.push(NewInt(int64(int32(binary.LittleEndian.Uint32(b)))))
	case opLong1:
		n, err := u.readByte()
		if err != nil {
			return u.truncated()
		}
		body, err := u.readN(int(n))
		if err != nil {
			return u.truncated()
		}
		u.push(NewIntFromBig(decodeLong(body)))
	case opLong4:
		lb, err := u.readN(4)
		if err != nil {
			return u.truncated()
		}
		body, err := u.readN(int(binary.LittleEndian.Uint32(lb)))
		if err != nil {
			return u.truncated()
		}
		u.push(NewIntFromBig(decodeLong(body)))
	case opBinFloat:
		b, err := u.readN(8)
		if err != nil {
			return u.truncated()
		}
		u.push(NewFloat(math.Float64frombits(binary.BigEndian.Uint64(b))))
	case opShortBinUnicode:
		n, err := u.readByte()
		if err != nil {
			return u.truncated()
		}
		return u.pushStr(int(n))
	case opBinUnicode:
		b, err := u.readN(4)
		if err != nil {
			return u.truncated()
		}
		return u.pushStr(int(binary.LittleEndian.Uint32(b)))
	case opBinUnicode8:
		b, err := u.readN(8)
		if err != nil {
			return u.truncated()
		}
		return u.pushStr(int(binary.LittleEndian.Uint64(b)))
	case opShortBinBytes:
		n, err := u.readByte()
		if err != nil {
			return u.truncated()
		}
		return u.pushBytes(int(n))
	case opBinBytes:
		b, err := u.readN(4)
		if err != nil {
			return u.truncated()
		}
		return u.pushBytes(int(binary.LittleEndian.Uint32(b)))
	case opBinBytes8:
		b, err := u.readN(8)
		if err != nil {
			return u.truncated()
		}
		return u.pushBytes(int(binary.LittleEndian.Uint64(b)))
	case opMemoize:
		if len(u.stack) == 0 {
			return newUnpicklingError("memoize on empty stack")
		}
		u.memo = append(u.memo, u.stack[len(u.stack)-1])
	case opBinPut:
		b, err := u.readByte()
		if err != nil {
			return u.truncated()
		}
		return u.putAt(int(b))
	case opLongBinPut:
		b, err := u.readN(4)
		if err != nil {
			return u.truncated()
		}
		return u.putAt(int(binary.LittleEndian.Uint32(b)))
	case opBinGet:
		b, err := u.readByte()
		if err != nil {
			return u.truncated()
		}
		return u.getAt(int(b))
	case opLongBinGet:
		b, err := u.readN(4)
		if err != nil {
			return u.truncated()
		}
		return u.getAt(int(binary.LittleEndian.Uint32(b)))
	case opMark:
		u.marks = append(u.marks, len(u.stack))
	case opEmptyTuple:
		u.push(NewTuple(nil))
	case opTuple1:
		return u.buildTuple(1)
	case opTuple2:
		return u.buildTuple(2)
	case opTuple3:
		return u.buildTuple(3)
	case opTuple:
		return u.buildTupleFromMark()
	case opEmptyList:
		u.push(NewList(nil))
	case opAppend:
		return u.appendOne()
	case opAppends:
		return u.appendFromMark()
	case opEmptyDict:
		d, err := NewDict(nil, nil)
		if err != nil {
			return err
		}
		u.push(d)
	case opSetItem:
		return u.setItemOne()
	case opSetItems:
		return u.setItemsFromMark()
	case opEmptySet:
		u.push(&setObject{newSetCore(0)})
	case opAddItems:
		return u.addItemsFromMark()
	case opFrozenset:
		return u.buildFrozensetFromMark()
	case opGlobal:
		return u.loadGlobal()
	case opStackGlobal:
		return u.loadStackGlobal()
	case opReduce:
		return u.reduce()
	case opNewObj:
		return u.newObj()
	case opNewObjEx:
		return u.newObjEx()
	case opBuild:
		return u.build()
	default:
		return newUnpicklingError("unsupported pickle opcode: 0x%02x", op)
	}
	return nil
}

// buildTuple pops the top n items and pushes them as a tuple.
func (u *unpickler) buildTuple(n int) error {
	if len(u.stack) < n {
		return newUnpicklingError("tuple build underflow")
	}
	at := len(u.stack) - n
	elts := make([]Object, n)
	copy(elts, u.stack[at:])
	u.stack = u.stack[:at]
	u.push(NewTuple(elts))
	return nil
}

// popMark returns the stack position of the innermost mark, removing it.
func (u *unpickler) popMark() (int, error) {
	if len(u.marks) == 0 {
		return 0, newUnpicklingError("no mark for a variable-length build")
	}
	at := u.marks[len(u.marks)-1]
	u.marks = u.marks[:len(u.marks)-1]
	if at > len(u.stack) {
		return 0, newUnpicklingError("mark past the stack top")
	}
	return at, nil
}

// buildTupleFromMark pops everything back to the mark and pushes it as a tuple.
func (u *unpickler) buildTupleFromMark() error {
	at, err := u.popMark()
	if err != nil {
		return err
	}
	elts := make([]Object, len(u.stack)-at)
	copy(elts, u.stack[at:])
	u.stack = u.stack[:at]
	u.push(NewTuple(elts))
	return nil
}

// appendOne appends the top item to the list beneath it, mutating in place so a
// memoized (shared or cyclic) list keeps its identity.
func (u *unpickler) appendOne() error {
	if len(u.stack) < 2 {
		return newUnpicklingError("append underflow")
	}
	item := u.stack[len(u.stack)-1]
	u.stack = u.stack[:len(u.stack)-1]
	lst, ok := u.stack[len(u.stack)-1].(*listObject)
	if !ok {
		return newUnpicklingError("append onto a non-list")
	}
	lst.elts = append(lst.elts, item)
	return nil
}

// appendFromMark appends everything back to the mark onto the list beneath it.
func (u *unpickler) appendFromMark() error {
	at, err := u.popMark()
	if err != nil {
		return err
	}
	items := u.stack[at:]
	if at == 0 {
		return newUnpicklingError("appends with no list under the mark")
	}
	lst, ok := u.stack[at-1].(*listObject)
	if !ok {
		return newUnpicklingError("appends onto a non-list")
	}
	lst.elts = append(lst.elts, items...)
	u.stack = u.stack[:at]
	return nil
}

// setItemOne sets the top key/value pair on the dict beneath them.
func (u *unpickler) setItemOne() error {
	if len(u.stack) < 3 {
		return newUnpicklingError("setitem underflow")
	}
	val := u.stack[len(u.stack)-1]
	key := u.stack[len(u.stack)-2]
	u.stack = u.stack[:len(u.stack)-2]
	d, ok := u.stack[len(u.stack)-1].(*dictObject)
	if !ok {
		return newUnpicklingError("setitem onto a non-dict")
	}
	return d.set(key, val)
}

// setItemsFromMark sets every key/value pair back to the mark on the dict
// beneath them, in order.
func (u *unpickler) setItemsFromMark() error {
	at, err := u.popMark()
	if err != nil {
		return err
	}
	if (len(u.stack)-at)%2 != 0 {
		return newUnpicklingError("setitems with an odd number of items")
	}
	if at == 0 {
		return newUnpicklingError("setitems with no dict under the mark")
	}
	d, ok := u.stack[at-1].(*dictObject)
	if !ok {
		return newUnpicklingError("setitems onto a non-dict")
	}
	pairs := u.stack[at:]
	for i := 0; i < len(pairs); i += 2 {
		if err := d.set(pairs[i], pairs[i+1]); err != nil {
			return err
		}
	}
	u.stack = u.stack[:at]
	return nil
}

// addItemsFromMark adds every element back to the mark to the set beneath them,
// mutating it in place so a memoized (shared) set keeps its identity.
func (u *unpickler) addItemsFromMark() error {
	at, err := u.popMark()
	if err != nil {
		return err
	}
	if at == 0 {
		return newUnpicklingError("additems with no set under the mark")
	}
	set, ok := u.stack[at-1].(*setObject)
	if !ok {
		return newUnpicklingError("additems onto a non-set")
	}
	for _, item := range u.stack[at:] {
		if err := set.addElt(item); err != nil {
			return err
		}
	}
	u.stack = u.stack[:at]
	return nil
}

// buildFrozensetFromMark pops everything back to the mark and pushes it as a
// frozenset.
func (u *unpickler) buildFrozensetFromMark() error {
	at, err := u.popMark()
	if err != nil {
		return err
	}
	elts := make([]Object, len(u.stack)-at)
	copy(elts, u.stack[at:])
	u.stack = u.stack[:at]
	f, err := NewFrozenset(elts)
	if err != nil {
		return err
	}
	u.push(f)
	return nil
}

// loadGlobal reads the newline-terminated module and qualname of a GLOBAL
// opcode, resolves them through find_class, and pushes the result.
func (u *unpickler) loadGlobal() error {
	module, err := u.readLine()
	if err != nil {
		return err
	}
	name, err := u.readLine()
	if err != nil {
		return err
	}
	g, err := u.findClass(module, name)
	if err != nil {
		return err
	}
	u.push(g)
	return nil
}

// loadStackGlobal takes the qualname and module a STACK_GLOBAL opcode left on
// the stack (module pushed first, qualname on top), resolves them, and pushes
// the result.
func (u *unpickler) loadStackGlobal() error {
	if len(u.stack) < 2 {
		return newUnpicklingError("stack_global underflow")
	}
	name, ok := u.stack[len(u.stack)-1].(*strObject)
	if !ok {
		return newUnpicklingError("stack_global qualname is not a string")
	}
	module, ok := u.stack[len(u.stack)-2].(*strObject)
	if !ok {
		return newUnpicklingError("stack_global module is not a string")
	}
	u.stack = u.stack[:len(u.stack)-2]
	g, err := u.findClass(module.v, name.v)
	if err != nil {
		return err
	}
	u.push(g)
	return nil
}

// findClass resolves a global by module and qualname to the value the pickler
// referenced. Old pickles name Python-2 modules, so the module is mapped forward
// first. This slice records the reference as a pickleGlobalRef that REDUCE turns
// back into an object; a later slice resolves globals used outside a reduction.
func (u *unpickler) findClass(module, name string) (Object, error) {
	if m, ok := compatForwardImport[module]; ok {
		module = m
	}
	if c := lookupPickleClass(module, name); c != nil {
		return c, nil
	}
	if fn := lookupPickleFunction(module, name); fn != nil {
		return fn, nil
	}
	return &pickleGlobalRef{module: module, qualname: name}, nil
}

// newObj rebuilds the instance a NEWOBJ opcode describes from the class and the
// new-arguments tuple on the stack: cls.__new__(cls, *args), without __init__.
func (u *unpickler) newObj() error {
	if len(u.stack) < 2 {
		return newUnpicklingError("newobj underflow")
	}
	argsObj := u.stack[len(u.stack)-1]
	clsObj := u.stack[len(u.stack)-2]
	u.stack = u.stack[:len(u.stack)-2]
	args, ok := argsObj.(*tupleObject)
	if !ok {
		return newUnpicklingError("newobj arguments are not a tuple")
	}
	cls, ok := clsObj.(*classObject)
	if !ok {
		return newUnpicklingError("newobj on a non-class")
	}
	obj, err := pickleNewInstance(cls, args.elts)
	if err != nil {
		return err
	}
	u.push(obj)
	return nil
}

// newObjEx rebuilds the instance a NEWOBJ_EX opcode describes, calling
// cls.__new__(cls, *args, **kwargs) from the class, argument tuple, and keyword
// dict on the stack, in that stacking order.
func (u *unpickler) newObjEx() error {
	if len(u.stack) < 3 {
		return newUnpicklingError("newobj_ex underflow")
	}
	kwargsObj := u.stack[len(u.stack)-1]
	argsObj := u.stack[len(u.stack)-2]
	clsObj := u.stack[len(u.stack)-3]
	u.stack = u.stack[:len(u.stack)-3]
	kwargs, ok := kwargsObj.(*dictObject)
	if !ok {
		return newUnpicklingError("newobj_ex keyword arguments are not a dict")
	}
	args, ok := argsObj.(*tupleObject)
	if !ok {
		return newUnpicklingError("newobj_ex arguments are not a tuple")
	}
	cls, ok := clsObj.(*classObject)
	if !ok {
		return newUnpicklingError("newobj_ex on a non-class")
	}
	obj, err := pickleNewInstanceEx(cls, args.elts, kwargs)
	if err != nil {
		return err
	}
	u.push(obj)
	return nil
}

// build applies the state on top of the stack to the instance below it, the way
// BUILD restores an instance's __dict__.
func (u *unpickler) build() error {
	if len(u.stack) < 2 {
		return newUnpicklingError("build underflow")
	}
	state := u.stack[len(u.stack)-1]
	u.stack = u.stack[:len(u.stack)-1]
	obj := u.stack[len(u.stack)-1]
	return pickleApplyState(obj, state)
}

// reduce applies the callable a REDUCE opcode leaves below its argument tuple,
// replacing both with the result.
func (u *unpickler) reduce() error {
	if len(u.stack) < 2 {
		return newUnpicklingError("reduce underflow")
	}
	argsObj := u.stack[len(u.stack)-1]
	fn := u.stack[len(u.stack)-2]
	u.stack = u.stack[:len(u.stack)-2]
	args, ok := argsObj.(*tupleObject)
	if !ok {
		return newUnpicklingError("reduce argument is not a tuple")
	}
	switch f := fn.(type) {
	case *pickleGlobalRef:
		// A builtin reducer resolved as a stand-in ref (set/frozenset) rebuilds
		// through its dedicated constructor.
		res, err := reduceGlobal(f, args)
		if err != nil {
			return err
		}
		u.push(res)
		return nil
	case *functionObject, *classObject:
		// A user reduction names a real module-level function or class, registered
		// as its module executed; applying it to the argument tuple rebuilds the
		// object, the same call CPython makes when it resolves the global by import.
		res, err := Call(fn, args.elts)
		if err != nil {
			return err
		}
		u.push(res)
		return nil
	}
	return newUnpicklingError("reduce on a non-callable %s is not supported", fn.TypeName())
}

// readLine reads bytes through the next newline, returning the text without it,
// for the GLOBAL opcode's module and qualname fields.
func (u *unpickler) readLine() (string, error) {
	start := u.pos
	for u.pos < len(u.data) {
		if u.data[u.pos] == '\n' {
			s := string(u.data[start:u.pos])
			u.pos++
			return s, nil
		}
		u.pos++
	}
	return "", u.truncated()
}

func (u *unpickler) push(o Object) { u.stack = append(u.stack, o) }

// pushStr reads n UTF-8 bytes and pushes them as a str.
func (u *unpickler) pushStr(n int) error {
	b, err := u.readN(n)
	if err != nil {
		return u.truncated()
	}
	u.push(NewStr(string(b)))
	return nil
}

// pushBytes reads n bytes and pushes them as a bytes object, copying so the
// pushed value does not alias the input buffer.
func (u *unpickler) pushBytes(n int) error {
	b, err := u.readN(n)
	if err != nil {
		return u.truncated()
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	u.push(NewBytes(cp))
	return nil
}

// putAt stores the top of stack at an explicit memo index (protocol 2/3),
// growing the memo as needed.
func (u *unpickler) putAt(idx int) error {
	if len(u.stack) == 0 {
		return newUnpicklingError("put on empty stack")
	}
	for len(u.memo) <= idx {
		u.memo = append(u.memo, nil)
	}
	u.memo[idx] = u.stack[len(u.stack)-1]
	return nil
}

// getAt pushes the memoized value at idx.
func (u *unpickler) getAt(idx int) error {
	if idx < 0 || idx >= len(u.memo) || u.memo[idx] == nil {
		return newUnpicklingError("memo value not found at index %d", idx)
	}
	u.push(u.memo[idx])
	return nil
}

func (u *unpickler) truncated() error { return newUnpicklingError("pickle data was truncated") }

// readByte returns the next byte and advances.
func (u *unpickler) readByte() (byte, error) {
	if u.pos >= len(u.data) {
		return 0, errPickleEOF
	}
	b := u.data[u.pos]
	u.pos++
	return b, nil
}

// readN returns the next n bytes as a sub-slice of the input and advances.
func (u *unpickler) readN(n int) ([]byte, error) {
	if n < 0 || u.pos+n > len(u.data) {
		return nil, errPickleEOF
	}
	b := u.data[u.pos : u.pos+n]
	u.pos += n
	return b, nil
}

// errPickleEOF is the sentinel readByte/readN return past the end; callers turn
// it into the UnpicklingError CPython raises for a truncated stream.
var errPickleEOF = newUnpicklingErrorSentinel()

func newUnpicklingErrorSentinel() error { return &pickleEOF{} }

type pickleEOF struct{}

func (*pickleEOF) Error() string { return "pickle EOF" }

// decodeLong reverses encodeLong: a little-endian two's-complement body back to
// a big integer, with an empty body meaning zero.
func decodeLong(body []byte) *big.Int {
	if len(body) == 0 {
		return big.NewInt(0)
	}
	// Reassemble the magnitude big-endian, then apply the sign from the top bit.
	be := make([]byte, len(body))
	for i := range body {
		be[len(body)-1-i] = body[i]
	}
	n := new(big.Int).SetBytes(be)
	if body[len(body)-1]&0x80 != 0 {
		mod := new(big.Int).Lsh(big.NewInt(1), uint(8*len(body)))
		n.Sub(n, mod)
	}
	return n
}
