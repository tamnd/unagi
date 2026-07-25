//go:build darwin || linux

package runtime

import (
	"bytes"
	"syscall"

	"github.com/tamnd/unagi/pkg/objects"
)

// mmap is the memory-mapped-file accelerator. CPython implements it as a C type
// with the buffer and sequence protocols; here it is an ordinary class whose
// methods are Go functions over syscall.Mmap, so an instance is backed by a real
// mapping and indexing, slicing, read/write, seek/tell, find and flush behave the
// way the C mmap does. It is scoped to darwin and linux, the hosts with the POSIX
// mmap(2) surface; Windows CPython has a separate CreateFileMapping path this
// slice does not cover.
//
// The native mapping lives in an mmapHandle stashed on the instance under a
// private attribute, the same shape the socket type uses. The access argument
// picks the protection and sharing: ACCESS_READ maps read-only shared,
// ACCESS_WRITE read-write shared (writes reach the file), ACCESS_COPY read-write
// private (copy-on-write), and ACCESS_DEFAULT honors the explicit prot/flags.

func init() {
	moduleTable["mmap"] = &moduleEntry{builtin: true, exec: initMmap}
}

// The access modes are CPython's own small integers, host-invariant.
const (
	mmapAccessDefault = 0
	mmapAccessRead    = 1
	mmapAccessWrite   = 2
	mmapAccessCopy    = 3
)

const mmapHandleAttr = "_unagi_mmap"

// mmapHandle is the native state of a mapping: the mapped bytes, the read/write
// cursor, the access mode, and whether it has been unmapped.
type mmapHandle struct {
	data     []byte
	pos      int
	access   int
	writable bool
	closed   bool
}

func (*mmapHandle) TypeName() string { return "mmap" }

func initMmap(m *objects.Module) error {
	set := func(name string, v objects.Object) error {
		return objects.StoreAttr(m, name, v)
	}
	cls, err := buildMmapClass()
	if err != nil {
		return err
	}
	if err := set("mmap", cls); err != nil {
		return err
	}
	pagesize := syscall.Getpagesize()
	consts := []struct {
		name string
		val  int
	}{
		{"ACCESS_DEFAULT", mmapAccessDefault},
		{"ACCESS_READ", mmapAccessRead},
		{"ACCESS_WRITE", mmapAccessWrite},
		{"ACCESS_COPY", mmapAccessCopy},
		{"PROT_READ", syscall.PROT_READ},
		{"PROT_WRITE", syscall.PROT_WRITE},
		{"PROT_EXEC", syscall.PROT_EXEC},
		{"MAP_SHARED", syscall.MAP_SHARED},
		{"MAP_PRIVATE", syscall.MAP_PRIVATE},
		{"MAP_ANON", syscall.MAP_ANON},
		{"MAP_ANONYMOUS", syscall.MAP_ANON},
		{"PAGESIZE", pagesize},
		{"ALLOCATIONGRANULARITY", pagesize},
	}
	for _, c := range consts {
		if err := set(c.name, objects.NewInt(int64(c.val))); err != nil {
			return err
		}
	}
	return nil
}

// buildMmapClass builds the mmap.mmap class: self-bound builtin methods over the
// native handle, including the sequence dunders (__len__/__getitem__/__setitem__)
// and the context-manager pair so `with mmap.mmap(...) as mm:` works.
func buildMmapClass() (objects.Object, error) {
	names := []string{
		"__slots__",
		"__init__",
		"__len__", "__getitem__", "__setitem__",
		"__enter__", "__exit__",
		"close", "closed",
		"read", "read_byte", "readline", "write", "write_byte",
		"seek", "tell", "size", "find", "rfind", "flush", "move",
	}
	vals := []objects.Object{
		objects.NewList([]objects.Object{objects.NewStr("__dict__")}),
		objects.NewMethodKw("__init__", mmapInit),
		objects.NewMethod("__len__", 1, mmapLen),
		objects.NewMethod("__getitem__", 2, mmapGetItem),
		objects.NewMethod("__setitem__", 3, mmapSetItem),
		objects.NewMethod("__enter__", 1, mmapEnter),
		objects.NewMethod("__exit__", -1, mmapExit),
		objects.NewMethod("close", 1, mmapClose),
		objects.NewProperty(objects.NewFunc("closed", 1, mmapClosedGet), nil, nil),
		objects.NewMethod("read", -1, mmapRead),
		objects.NewMethod("read_byte", 1, mmapReadByte),
		objects.NewMethod("readline", 1, mmapReadline),
		objects.NewMethod("write", 2, mmapWrite),
		objects.NewMethod("write_byte", 2, mmapWriteByte),
		objects.NewMethod("seek", -1, mmapSeek),
		objects.NewMethod("tell", 1, mmapTell),
		objects.NewMethod("size", 1, mmapSize),
		objects.NewMethod("find", -1, mmapFind),
		objects.NewMethod("rfind", -1, mmapRfind),
		objects.NewMethod("flush", -1, mmapFlush),
		objects.NewMethod("move", 4, mmapMove),
	}
	return objects.NewClass("mmap", "mmap.mmap", nil, names, vals, nil, nil)
}

// mmapArg resolves the constructor arguments, honoring both position and the
// fileno/length/flags/prot/access/offset keyword names CPython accepts.
func mmapArg(pos []objects.Object, kwNames []string, kwVals []objects.Object, index int, name string) (objects.Object, bool) {
	for i, kn := range kwNames {
		if kn == name {
			return kwVals[i], true
		}
	}
	if index < len(pos) {
		return pos[index], true
	}
	return nil, false
}

// mmapInit is mmap.mmap.__init__(self, fileno, length, flags=MAP_SHARED,
// prot=PROT_READ|PROT_WRITE, access=ACCESS_DEFAULT, offset=0). A fileno of -1 is
// an anonymous mapping; a length of 0 with a real file maps the whole file from
// the offset.
func mmapInit(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 1 {
		return nil, objects.Raise(objects.TypeError, "__init__ needs self")
	}
	self := pos[0]
	rest := pos[1:]

	intArg := func(index int, name string, def int64) (int64, error) {
		v, ok := mmapArg(rest, kwNames, kwVals, index, name)
		if !ok {
			return def, nil
		}
		n, ok := objects.AsInt(v)
		if !ok {
			return 0, objects.Raise(objects.TypeError, "mmap() %s must be an integer", name)
		}
		return n, nil
	}

	fileno, err := intArg(0, "fileno", -1)
	if err != nil {
		return nil, err
	}
	length, err := intArg(1, "length", 0)
	if err != nil {
		return nil, err
	}
	flags, err := intArg(2, "flags", int64(syscall.MAP_SHARED))
	if err != nil {
		return nil, err
	}
	prot, err := intArg(3, "prot", int64(syscall.PROT_READ|syscall.PROT_WRITE))
	if err != nil {
		return nil, err
	}
	access, err := intArg(4, "access", mmapAccessDefault)
	if err != nil {
		return nil, err
	}
	offset, err := intArg(5, "offset", 0)
	if err != nil {
		return nil, err
	}

	// The access mode, when not default, dictates the protection and sharing,
	// overriding prot/flags exactly as CPython's mmap does.
	writable := prot&syscall.PROT_WRITE != 0
	switch access {
	case mmapAccessRead:
		prot, flags, writable = syscall.PROT_READ, syscall.MAP_SHARED, false
	case mmapAccessWrite:
		prot, flags, writable = syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED, true
	case mmapAccessCopy:
		prot, flags, writable = syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_PRIVATE, true
	case mmapAccessDefault:
		// prot/flags/writable stay as given.
	default:
		return nil, objects.Raise(objects.ValueError, "mmap invalid access parameter")
	}

	if length < 0 {
		return nil, objects.Raise(objects.OverflowError, "memory mapped length must be positive")
	}

	fd := int(fileno)
	if fd == -1 {
		// Anonymous mapping: no file, so it must have a size.
		if length == 0 {
			return nil, objects.Raise(objects.ValueError, "cannot mmap an empty file")
		}
		flags |= syscall.MAP_ANON
	} else if length == 0 {
		// Whole-file mapping: take the size from the descriptor.
		var st syscall.Stat_t
		if err := syscall.Fstat(fd, &st); err != nil {
			return nil, posixStatErr(err)
		}
		length = st.Size - offset
		if length <= 0 {
			return nil, objects.Raise(objects.ValueError, "cannot mmap an empty file")
		}
	}

	data, err := syscall.Mmap(fd, offset, int(length), int(prot), int(flags))
	if err != nil {
		return nil, posixStatErr(err)
	}
	h := &mmapHandle{data: data, access: int(access), writable: writable}
	if err := objects.StoreAttr(self, mmapHandleAttr, h); err != nil {
		_ = syscall.Munmap(data)
		return nil, err
	}
	return objects.None, nil
}

// mmapOf reads the native handle for an operation that needs a live mapping,
// raising the ValueError CPython gives once the mapping is closed.
func mmapOf(self objects.Object) (*mmapHandle, error) {
	v, err := objects.LoadAttr(self, mmapHandleAttr)
	if err != nil {
		return nil, objects.Raise(objects.TypeError, "not an mmap object")
	}
	h, ok := v.(*mmapHandle)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "not an mmap object")
	}
	if h.closed {
		return nil, objects.Raise(objects.ValueError, "mmap closed or invalid")
	}
	return h, nil
}

// mmapNeedWritable raises the TypeError CPython gives when a read-only mapping is
// written through.
func mmapNeedWritable(h *mmapHandle) error {
	if !h.writable {
		return objects.Raise(objects.TypeError, "mmap can't modify a readonly memory map.")
	}
	return nil
}

func mmapLen(args []objects.Object) (objects.Object, error) {
	h, err := mmapOf(args[0])
	if err != nil {
		return nil, err
	}
	return objects.NewInt(int64(len(h.data))), nil
}

func mmapGetItem(args []objects.Object) (objects.Object, error) {
	h, err := mmapOf(args[0])
	if err != nil {
		return nil, err
	}
	key := args[1]
	n := len(h.data)
	if key.TypeName() == "slice" {
		tup, err := objects.CallMethod(key, "indices", []objects.Object{objects.NewInt(int64(n))})
		if err != nil {
			return nil, err
		}
		parts, err := objects.IterToSlice(tup)
		if err != nil || len(parts) != 3 {
			return nil, objects.Raise(objects.TypeError, "bad slice")
		}
		start, _ := objects.AsInt(parts[0])
		stop, _ := objects.AsInt(parts[1])
		step, _ := objects.AsInt(parts[2])
		var out []byte
		if step == 1 {
			if start < stop {
				out = append(out, h.data[start:stop]...)
			}
		} else {
			for i := start; (step > 0 && i < stop) || (step < 0 && i > stop); i += step {
				out = append(out, h.data[i])
			}
		}
		return objects.NewBytes(out), nil
	}
	idx, ok := objects.AsInt(key)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "mmap indices must be integers")
	}
	i := int(idx)
	if i < 0 {
		i += n
	}
	if i < 0 || i >= n {
		return nil, objects.Raise(objects.IndexError, "mmap index out of range")
	}
	return objects.NewInt(int64(h.data[i])), nil
}

func mmapSetItem(args []objects.Object) (objects.Object, error) {
	h, err := mmapOf(args[0])
	if err != nil {
		return nil, err
	}
	if err := mmapNeedWritable(h); err != nil {
		return nil, err
	}
	key, val := args[1], args[2]
	n := len(h.data)
	if key.TypeName() == "slice" {
		tup, err := objects.CallMethod(key, "indices", []objects.Object{objects.NewInt(int64(n))})
		if err != nil {
			return nil, err
		}
		parts, _ := objects.IterToSlice(tup)
		start, _ := objects.AsInt(parts[0])
		stop, _ := objects.AsInt(parts[1])
		step, _ := objects.AsInt(parts[2])
		b, ok := objects.AsBytesLike(val)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "mmap slice assignment must be bytes")
		}
		// CPython requires the slice and the value to be the same length.
		var count int64
		if step == 1 {
			count = stop - start
			if count < 0 {
				count = 0
			}
		} else {
			for i := start; (step > 0 && i < stop) || (step < 0 && i > stop); i += step {
				count++
			}
		}
		if int64(len(b)) != count {
			return nil, objects.Raise(objects.IndexError, "mmap slice assignment is wrong size")
		}
		j := 0
		if step == 1 {
			copy(h.data[start:stop], b)
		} else {
			for i := start; (step > 0 && i < stop) || (step < 0 && i > stop); i += step {
				h.data[i] = b[j]
				j++
			}
		}
		return objects.None, nil
	}
	idx, ok := objects.AsInt(key)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "mmap indices must be integers")
	}
	i := int(idx)
	if i < 0 {
		i += n
	}
	if i < 0 || i >= n {
		return nil, objects.Raise(objects.IndexError, "mmap index out of range")
	}
	v, ok := objects.AsInt(val)
	if !ok || v < 0 || v > 255 {
		return nil, objects.Raise(objects.ValueError, "mmap item value must be in range(0, 256)")
	}
	h.data[i] = byte(v)
	return objects.None, nil
}

func mmapEnter(args []objects.Object) (objects.Object, error) {
	if _, err := mmapOf(args[0]); err != nil {
		return nil, err
	}
	return args[0], nil
}

func mmapExit(args []objects.Object) (objects.Object, error) {
	return mmapClose(args[:1])
}

func mmapClose(args []objects.Object) (objects.Object, error) {
	v, err := objects.LoadAttr(args[0], mmapHandleAttr)
	if err != nil {
		return objects.None, nil
	}
	h, ok := v.(*mmapHandle)
	if !ok || h.closed {
		return objects.None, nil
	}
	h.closed = true
	_ = syscall.Munmap(h.data)
	h.data = nil
	return objects.None, nil
}

func mmapClosedGet(args []objects.Object) (objects.Object, error) {
	v, err := objects.LoadAttr(args[0], mmapHandleAttr)
	if err != nil {
		return objects.True, nil
	}
	h, ok := v.(*mmapHandle)
	if !ok || h.closed {
		return objects.True, nil
	}
	return objects.False, nil
}

func mmapRead(args []objects.Object) (objects.Object, error) {
	h, err := mmapOf(args[0])
	if err != nil {
		return nil, err
	}
	n := len(h.data) - h.pos
	if len(args) >= 2 && args[1] != objects.None {
		want, ok := objects.AsInt(args[1])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "read() argument must be an integer")
		}
		if want >= 0 && int(want) < n {
			n = int(want)
		}
	}
	out := make([]byte, n)
	copy(out, h.data[h.pos:h.pos+n])
	h.pos += n
	return objects.NewBytes(out), nil
}

func mmapReadByte(args []objects.Object) (objects.Object, error) {
	h, err := mmapOf(args[0])
	if err != nil {
		return nil, err
	}
	if h.pos >= len(h.data) {
		return nil, objects.Raise(objects.ValueError, "read byte out of range")
	}
	b := h.data[h.pos]
	h.pos++
	return objects.NewInt(int64(b)), nil
}

func mmapReadline(args []objects.Object) (objects.Object, error) {
	h, err := mmapOf(args[0])
	if err != nil {
		return nil, err
	}
	start := h.pos
	end := len(h.data)
	if i := bytes.IndexByte(h.data[start:], '\n'); i >= 0 {
		end = start + i + 1
	}
	out := make([]byte, end-start)
	copy(out, h.data[start:end])
	h.pos = end
	return objects.NewBytes(out), nil
}

func mmapWrite(args []objects.Object) (objects.Object, error) {
	h, err := mmapOf(args[0])
	if err != nil {
		return nil, err
	}
	if err := mmapNeedWritable(h); err != nil {
		return nil, err
	}
	b, ok := objects.AsBytesLike(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "data must be a bytes-like object")
	}
	if h.pos+len(b) > len(h.data) {
		return nil, objects.Raise(objects.ValueError, "data out of range")
	}
	copy(h.data[h.pos:], b)
	h.pos += len(b)
	return objects.NewInt(int64(len(b))), nil
}

func mmapWriteByte(args []objects.Object) (objects.Object, error) {
	h, err := mmapOf(args[0])
	if err != nil {
		return nil, err
	}
	if err := mmapNeedWritable(h); err != nil {
		return nil, err
	}
	v, ok := objects.AsInt(args[1])
	if !ok || v < 0 || v > 255 {
		return nil, objects.Raise(objects.ValueError, "mmap item value must be in range(0, 256)")
	}
	if h.pos >= len(h.data) {
		return nil, objects.Raise(objects.ValueError, "write byte out of range")
	}
	h.data[h.pos] = byte(v)
	h.pos++
	return objects.None, nil
}

func mmapSeek(args []objects.Object) (objects.Object, error) {
	h, err := mmapOf(args[0])
	if err != nil {
		return nil, err
	}
	if len(args) < 2 {
		return nil, objects.Raise(objects.TypeError, "seek() takes at least 1 argument")
	}
	dist, ok := objects.AsInt(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "seek() argument must be an integer")
	}
	whence := int64(0)
	if len(args) >= 3 && args[2] != objects.None {
		w, ok := objects.AsInt(args[2])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "seek() whence must be an integer")
		}
		whence = w
	}
	var target int64
	switch whence {
	case 0:
		target = dist
	case 1:
		target = int64(h.pos) + dist
	case 2:
		target = int64(len(h.data)) + dist
	default:
		return nil, objects.Raise(objects.ValueError, "unknown seek type")
	}
	if target < 0 || target > int64(len(h.data)) {
		return nil, objects.Raise(objects.ValueError, "seek out of range")
	}
	h.pos = int(target)
	return objects.None, nil
}

func mmapTell(args []objects.Object) (objects.Object, error) {
	h, err := mmapOf(args[0])
	if err != nil {
		return nil, err
	}
	return objects.NewInt(int64(h.pos)), nil
}

func mmapSize(args []objects.Object) (objects.Object, error) {
	h, err := mmapOf(args[0])
	if err != nil {
		return nil, err
	}
	return objects.NewInt(int64(len(h.data))), nil
}

// mmapFindWith is the shared body of find and rfind: the byte offset of sub in
// the [start, end) window, or -1, searching forward or backward.
func mmapFindWith(name string, reverse bool, args []objects.Object) (objects.Object, error) {
	h, err := mmapOf(args[0])
	if err != nil {
		return nil, err
	}
	if len(args) < 2 {
		return nil, objects.Raise(objects.TypeError, "%s() takes at least 1 argument", name)
	}
	sub, ok := objects.AsBytesLike(args[1])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "%s() argument must be bytes-like", name)
	}
	n := len(h.data)
	start := h.pos
	end := n
	if len(args) >= 3 && args[2] != objects.None {
		if v, ok := objects.AsInt(args[2]); ok {
			start = clampIndex(int(v), n)
		}
	}
	if len(args) >= 4 && args[3] != objects.None {
		if v, ok := objects.AsInt(args[3]); ok {
			end = clampIndex(int(v), n)
		}
	}
	if start > end {
		return objects.NewInt(-1), nil
	}
	window := h.data[start:end]
	var rel int
	if reverse {
		rel = bytes.LastIndex(window, sub)
	} else {
		rel = bytes.Index(window, sub)
	}
	if rel < 0 {
		return objects.NewInt(-1), nil
	}
	return objects.NewInt(int64(start + rel)), nil
}

// clampIndex normalizes a possibly-negative find bound to [0, n], the way
// CPython clamps the start and end arguments.
func clampIndex(i, n int) int {
	if i < 0 {
		i += n
	}
	if i < 0 {
		return 0
	}
	if i > n {
		return n
	}
	return i
}

func mmapFind(args []objects.Object) (objects.Object, error) {
	return mmapFindWith("find", false, args)
}

func mmapRfind(args []objects.Object) (objects.Object, error) {
	return mmapFindWith("rfind", true, args)
}

// mmapFlush is mmap.flush(offset=0, size=0). The mapping is shared memory, so the
// bytes are already coherent; there is nothing to synchronize into the process
// view, and CPython 3.x returns None.
func mmapFlush(args []objects.Object) (objects.Object, error) {
	if _, err := mmapOf(args[0]); err != nil {
		return nil, err
	}
	return objects.None, nil
}

// mmapMove is mmap.move(dest, src, count): copies count bytes within the mapping,
// overlap-safe like memmove.
func mmapMove(args []objects.Object) (objects.Object, error) {
	h, err := mmapOf(args[0])
	if err != nil {
		return nil, err
	}
	if err := mmapNeedWritable(h); err != nil {
		return nil, err
	}
	dest, ok1 := objects.AsInt(args[1])
	src, ok2 := objects.AsInt(args[2])
	count, ok3 := objects.AsInt(args[3])
	if !ok1 || !ok2 || !ok3 {
		return nil, objects.Raise(objects.TypeError, "move() arguments must be integers")
	}
	n := int64(len(h.data))
	if dest < 0 || src < 0 || count < 0 || dest+count > n || src+count > n {
		return nil, objects.Raise(objects.ValueError, "source, destination, or count out of range")
	}
	copy(h.data[dest:dest+count], h.data[src:src+count])
	return objects.None, nil
}
