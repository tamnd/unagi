package objects

import "strings"

// This file implements augmented assignment (x += y and friends) the way
// CPython's PyNumber_InPlace* functions do: try the left operand's in-place
// dunder, then the mutable-builtin in-place path so aliases observe the change,
// then the ordinary binary operator, and finally re-raise the unsupported
// operand error under the augmented symbol.

// augSpec describes one augmented operator: the in-place dunder a user class may
// define, the binary function used on fallback, and an optional in-place mutator
// for the mutable builtins (nil when no builtin type has an in-place slot).
type augSpec struct {
	dunder  string
	bin     func(a, b Object) (Object, error)
	builtin func(a, b Object) (Object, bool, error)
}

var augSpecs = map[string]augSpec{
	"+=":  {"__iadd__", Add, inplaceConcat},
	"-=":  {"__isub__", Sub, inplaceDiff},
	"*=":  {"__imul__", Mul, inplaceRepeat},
	"@=":  {"__imatmul__", MatMul, nil},
	"/=":  {"__itruediv__", TrueDiv, nil},
	"//=": {"__ifloordiv__", FloorDiv, nil},
	"%=":  {"__imod__", Mod, nil},
	"**=": {"__ipow__", Pow, nil},
	"<<=": {"__ilshift__", LShift, nil},
	">>=": {"__irshift__", RShift, nil},
	"&=":  {"__iand__", BitAnd, inplaceIntersect},
	"|=":  {"__ior__", BitOr, inplaceUnion},
	"^=":  {"__ixor__", BitXor, inplaceSymDiff},
}

// InPlace performs an augmented assignment operator. sym is the augmented
// spelling ("+=", "|=", ...). The result is the value to rebind to the target;
// for a mutable builtin or an in-place dunder returning self it is the same
// object, so aliases see the mutation.
func InPlace(sym string, a, b Object) (Object, error) {
	spec, ok := augSpecs[sym]
	if !ok {
		return nil, unsupported(sym, a, b)
	}
	if fn := instDunderFn(a, spec.dunder); fn != nil {
		res, err := fn.bind([]Object{a, b}, nil, nil)
		if err != nil {
			return nil, err
		}
		if res != NotImplemented {
			return res, nil
		}
	}
	if spec.builtin != nil {
		if res, ok, err := spec.builtin(a, b); ok || err != nil {
			return res, err
		}
	}
	res, err := spec.bin(a, b)
	if err != nil {
		return nil, remapAug(err, sym, a, b)
	}
	return res, nil
}

// remapAug rewrites the binary fallback's "unsupported operand type(s) for OP"
// error to name the augmented symbol, matching CPython which raises the
// augmented spelling when the in-place fallback finds no handler. Other
// TypeErrors (concatenation, non-int repeat) already read the same for both
// forms and pass through unchanged.
func remapAug(err error, sym string, a, b Object) error {
	if ex, ok := err.(*Exception); ok && ex.Kind == TypeError &&
		strings.HasPrefix(ex.Text(), "unsupported operand type(s) for ") {
		return unsupported(sym, a, b)
	}
	return err
}

// inplaceConcat handles list += iterable in place: it returns ok=true whenever
// the left operand is a list, extending it with every item of b (raising
// "'X' object is not iterable" like list.extend when b is not iterable) so a
// list += never falls back to binary concatenation and aliases see the growth.
func inplaceConcat(a, b Object) (Object, bool, error) {
	// bytearray += only accepts a bytes-like right operand (probed: += a list
	// or str raises "can't concat X to bytearray"), so a non-bytes-like operand
	// declines and the binary Add raises that exact concat error.
	if ba, ok := a.(*bytearrayObject); ok {
		bl, ok := asBytesLike(b)
		if !ok {
			return nil, false, nil
		}
		ba.mu.Lock()
		ba.v = append(ba.v, bl...)
		ba.mu.Unlock()
		return ba, true, nil
	}
	lst, ok := a.(*listObject)
	if !ok {
		return nil, false, nil
	}
	// Materialize first so a += a reads the original length, not the slice as
	// it grows.
	items, err := iterAll(b)
	if err != nil {
		return nil, true, err
	}
	lst.elts = append(lst.elts, items...)
	return lst, true, nil
}

// inplaceRepeat handles list *= n in place. A non-int right operand or a big
// count declines (ok=false) so the binary Mul raises the same non-int or
// overflow message it would for list * n.
func inplaceRepeat(a, b Object) (Object, bool, error) {
	if ba, ok := a.(*bytearrayObject); ok {
		n, ok := AsInt(b)
		if !ok {
			return nil, false, nil
		}
		ba.mu.Lock()
		if n <= 0 {
			ba.v = ba.v[:0]
		} else {
			base := append([]byte(nil), ba.v...)
			for i := int64(1); i < n; i++ {
				ba.v = append(ba.v, base...)
			}
		}
		ba.mu.Unlock()
		return ba, true, nil
	}
	lst, ok := a.(*listObject)
	if !ok {
		return nil, false, nil
	}
	n, ok := AsInt(b)
	if !ok {
		return nil, false, nil
	}
	if n <= 0 {
		lst.elts = lst.elts[:0]
		return lst, true, nil
	}
	base := append([]Object(nil), lst.elts...)
	for i := int64(1); i < n; i++ {
		lst.elts = append(lst.elts, base...)
	}
	return lst, true, nil
}

// iterAll drains an iterable into a slice, surfacing any iteration error.
func iterAll(o Object) ([]Object, error) {
	it, err := Iter(o)
	if err != nil {
		return nil, err
	}
	var out []Object
	for {
		v, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return out, nil
		}
		out = append(out, v)
	}
}

// The set in-place operators mutate a set in place when both operands are sets
// (or frozensets), matching CPython's set.__ior__ and friends. A frozenset left
// operand or a non-set right operand declines so the binary op rebinds (a new
// frozenset) or raises the augmented unsupported-operand error.
func inplaceUnion(a, b Object) (Object, bool, error) {
	// dict |= merges in place like dict.update, taking a mapping or an iterable
	// of pairs, which is wider than the binary dict | that needs two dicts.
	if d, ok := a.(*dictObject); ok {
		if err := dictUpdate(d, b); err != nil {
			return nil, true, err
		}
		return d, true, nil
	}
	return inplaceSet(a, b, func(dst, x, y *setCore) { unionInto(dst, x, y) })
}

func inplaceIntersect(a, b Object) (Object, bool, error) {
	return inplaceSet(a, b, intersectInto)
}

func inplaceDiff(a, b Object) (Object, bool, error) {
	return inplaceSet(a, b, diffInto)
}

func inplaceSymDiff(a, b Object) (Object, bool, error) {
	return inplaceSet(a, b, symDiffInto)
}

func inplaceSet(a, b Object, combine func(dst, x, y *setCore)) (Object, bool, error) {
	s, ok := a.(*setObject)
	if !ok {
		return nil, false, nil
	}
	bc, ok := asSetCore(b)
	if !ok {
		return nil, false, nil
	}
	out := newSetCore(len(s.keys) + len(bc.keys))
	combine(&out, &s.setCore, bc)
	s.setCore = out
	return s, true, nil
}
