package objects

import (
	"strings"
	"sync"
)

// bytearrayObject is a mutable bytes buffer. Its bytes grow list-style and
// every mutating method takes mu so concurrent mutations from goroutine
// threads stay atomic and race-free. Reads that snapshot the buffer copy
// the bytes under the same lock.
type bytearrayObject struct {
	mu sync.Mutex
	v  []byte
}

func (*bytearrayObject) TypeName() string { return "bytearray" }

// NewByteArray boxes a byte slice as a mutable bytearray. The caller hands
// off ownership of b; later mutation is guarded by the object's lock.
func NewByteArray(b []byte) Object { return &bytearrayObject{v: b} }

// snapshot returns a copy of the buffer taken under the lock, so an iterator
// or repr never observes a torn write from a concurrent mutation.
func (x *bytearrayObject) snapshot() []byte {
	x.mu.Lock()
	defer x.mu.Unlock()
	return append([]byte(nil), x.v...)
}

// bytearrayRepr renders a bytearray as bytearray(b'...'), reusing the bytes
// repr for the inner literal.
func bytearrayRepr(v []byte) string {
	return "bytearray(" + bytesRepr(v) + ")"
}

// asBytesLike returns the raw bytes behind a bytes or bytearray value. It is
// the shared accessor for the cross-type operators: b'x' == bytearray(b'x'),
// concatenation, membership and ordering all compare the underlying bytes
// regardless of which of the two types holds them.
func asBytesLike(o Object) ([]byte, bool) {
	switch x := o.(type) {
	case *bytesObject:
		return x.v, true
	case *bytearrayObject:
		return x.snapshot(), true
	}
	return nil, false
}

// byteFromObj coerces an object to a single byte, raising rangeMsg when an
// integer is out of range(0, 256) and the CPython not-an-integer TypeError
// for a non-integer. rangeMsg differs between the bytes constructor ("bytes
// must be in range(0, 256)") and everything else ("byte must be ...").
func byteFromObj(o Object, rangeMsg string) (byte, error) {
	if i, ok := AsInt(o); ok {
		if i < 0 || i > 255 {
			return 0, Raise(ValueError, "%s", rangeMsg)
		}
		return byte(i), nil
	}
	if IsBigInt(o) {
		return 0, Raise(ValueError, "%s", rangeMsg)
	}
	return 0, Raise(TypeError, "'%s' object cannot be interpreted as an integer", o.TypeName())
}

// bytesFromIter collects an iterable of ints into a byte slice, applying the
// byte-range check to each element. A bytes or bytearray argument iterates as
// its member ints, so it flows through here too.
func bytesFromIter(o Object, rangeMsg string) ([]byte, error) {
	it, err := Iter(o)
	if err != nil {
		return nil, err
	}
	var out []byte
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return out, nil
		}
		b, err := byteFromObj(v, rangeMsg)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
}

const byteRangeMsg = "byte must be in range(0, 256)"

// bytearrayMethod dispatches the mutable bytearray method surface. Every
// mutator takes the lock so a method is atomic against concurrent threads.
func bytearrayMethod(x *bytearrayObject, name string, args []Object) (Object, error) {
	switch name {
	case "append":
		if len(args) != 1 {
			return nil, Raise(TypeError, "append() takes exactly one argument (%d given)", len(args))
		}
		b, err := byteFromObj(args[0], byteRangeMsg)
		if err != nil {
			return nil, err
		}
		x.mu.Lock()
		x.v = append(x.v, b)
		x.mu.Unlock()
		return None, nil
	case "extend":
		if len(args) != 1 {
			return nil, Raise(TypeError, "extend() takes exactly one argument (%d given)", len(args))
		}
		add, err := bytesFromIter(args[0], byteRangeMsg)
		if err != nil {
			// A non-iterable argument gets the dedicated bytearray wording.
			if ex, ok := err.(*Exception); ok && ex.Kind == TypeError &&
				strings.Contains(ex.Text(), "not iterable") {
				return nil, Raise(TypeError, "can't extend bytearray with %s", args[0].TypeName())
			}
			return nil, err
		}
		x.mu.Lock()
		x.v = append(x.v, add...)
		x.mu.Unlock()
		return None, nil
	case "insert":
		if len(args) != 2 {
			return nil, Raise(TypeError, "insert() takes exactly 2 arguments (%d given)", len(args))
		}
		i, ok := AsInt(args[0])
		if !ok {
			if IsBigInt(args[0]) {
				return nil, errIndexFit()
			}
			return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
		}
		b, err := byteFromObj(args[1], byteRangeMsg)
		if err != nil {
			return nil, err
		}
		x.mu.Lock()
		pos := clampInsert(i, len(x.v))
		x.v = append(x.v, 0)
		copy(x.v[pos+1:], x.v[pos:])
		x.v[pos] = b
		x.mu.Unlock()
		return None, nil
	case "pop":
		if len(args) > 1 {
			return nil, Raise(TypeError, "pop() takes at most 1 argument (%d given)", len(args))
		}
		idx := int64(-1)
		if len(args) == 1 {
			i, ok := AsInt(args[0])
			if !ok {
				if IsBigInt(args[0]) {
					return nil, errIndexFit()
				}
				return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
			}
			idx = i
		}
		x.mu.Lock()
		defer x.mu.Unlock()
		if len(x.v) == 0 {
			return nil, Raise(IndexError, "pop from empty bytearray")
		}
		j, err := seqIndex(idx, len(x.v), "pop index out of range")
		if err != nil {
			return nil, err
		}
		out := x.v[j]
		x.v = append(x.v[:j], x.v[j+1:]...)
		return NewInt(int64(out)), nil
	case "remove":
		if len(args) != 1 {
			return nil, Raise(TypeError, "remove() takes exactly one argument (%d given)", len(args))
		}
		b, err := byteFromObj(args[0], byteRangeMsg)
		if err != nil {
			return nil, err
		}
		x.mu.Lock()
		defer x.mu.Unlock()
		for i, c := range x.v {
			if c == b {
				x.v = append(x.v[:i], x.v[i+1:]...)
				return None, nil
			}
		}
		return nil, Raise(ValueError, "value not found in bytearray")
	case "clear":
		if len(args) != 0 {
			return nil, Raise(TypeError, "clear() takes no arguments (%d given)", len(args))
		}
		x.mu.Lock()
		x.v = x.v[:0]
		x.mu.Unlock()
		return None, nil
	case "reverse":
		if len(args) != 0 {
			return nil, Raise(TypeError, "reverse() takes no arguments (%d given)", len(args))
		}
		x.mu.Lock()
		for i, j := 0, len(x.v)-1; i < j; i, j = i+1, j-1 {
			x.v[i], x.v[j] = x.v[j], x.v[i]
		}
		x.mu.Unlock()
		return None, nil
	case "copy":
		if len(args) != 0 {
			return nil, Raise(TypeError, "copy() takes no arguments (%d given)", len(args))
		}
		return NewByteArray(x.snapshot()), nil
	}
	// The read methods (count/find/startswith/hex/...) are shared with bytes.
	return bytesReadMethod(x.snapshot(), "bytearray", name, args)
}

// clampInsert normalizes an insert index the way bytearray.insert does:
// negatives count from the end and any out-of-range index clamps to a valid
// insertion point instead of raising.
func clampInsert(i int64, n int) int {
	if i < 0 {
		i += int64(n)
		if i < 0 {
			return 0
		}
	}
	if i > int64(n) {
		return n
	}
	return int(i)
}
