package objects

import (
	"sort"
	"strconv"
	"strings"
	"testing"
)

// sortedSetRepr renders a set of ints in ascending order, so the tests do not
// depend on the set's insertion-order iteration.
func sortedSetRepr(o Object) string {
	it, err := Iter(o)
	if err != nil {
		return "err:" + err.Error()
	}
	var nums []int
	for {
		v, ok, err := it.Next()
		if err != nil || !ok {
			break
		}
		n, _ := AsInt(v)
		nums = append(nums, int(n))
	}
	sort.Ints(nums)
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = strconv.Itoa(n)
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// list += extends the same list in place, so a second reference sees the
// growth and InPlace returns the identical object.
func TestInPlaceListConcat(t *testing.T) {
	a := NewList([]Object{NewInt(1)})
	res, err := InPlace("+=", a, NewList([]Object{NewInt(2), NewInt(3)}))
	if err != nil {
		t.Fatalf("InPlace += : %v", err)
	}
	if res != a {
		t.Fatalf("+= rebound to a new object, want the same list")
	}
	if got := Repr(a); got != "[1, 2, 3]" {
		t.Fatalf("after += list = %s", got)
	}
	// Any iterable works, matching list.extend.
	if _, err := InPlace("+=", a, NewStr("xy")); err != nil {
		t.Fatalf("+= str: %v", err)
	}
	if got := Repr(a); got != "[1, 2, 3, 'x', 'y']" {
		t.Fatalf("after += str list = %s", got)
	}
	// A non-iterable right operand raises list.extend's message, not a
	// concatenation error.
	_, err = InPlace("+=", NewList(nil), NewInt(5))
	if err == nil || err.Error() != "TypeError: 'int' object is not iterable" {
		t.Fatalf("+= int error = %v", err)
	}
}

// list *= repeats in place; a big or non-int count declines to the binary Mul.
func TestInPlaceListRepeat(t *testing.T) {
	a := NewList([]Object{NewInt(1), NewInt(2)})
	res, err := InPlace("*=", a, NewInt(3))
	if err != nil {
		t.Fatalf("InPlace *= : %v", err)
	}
	if res != a || Repr(a) != "[1, 2, 1, 2, 1, 2]" {
		t.Fatalf("after *= list = %s (same=%v)", Repr(a), res == a)
	}
	zero, _ := InPlace("*=", a, NewInt(0))
	if zero != a || Repr(a) != "[]" {
		t.Fatalf("*= 0 = %s", Repr(a))
	}
	_, err = InPlace("*=", NewList([]Object{NewInt(1)}), NewStr("z"))
	if err == nil || !strings.Contains(err.Error(), "can't multiply sequence") {
		t.Fatalf("*= str error = %v", err)
	}
}

// set operators update the set in place when both operands are sets.
func TestInPlaceSetOps(t *testing.T) {
	mk := func(vals ...int64) Object {
		elts := make([]Object, len(vals))
		for i, v := range vals {
			elts[i] = NewInt(v)
		}
		s, err := NewSet(elts)
		if err != nil {
			t.Fatalf("NewSet: %v", err)
		}
		return s
	}
	cases := []struct {
		op   string
		rhs  Object
		want string
	}{
		{"|=", mk(3, 4), "{1, 2, 3, 4}"},
		{"&=", mk(2, 3), "{2}"},
		{"^=", mk(2, 9), "{1, 9}"},
		{"-=", mk(1), "{2}"},
	}
	for _, c := range cases {
		s := mk(1, 2)
		res, err := InPlace(c.op, s, c.rhs)
		if err != nil {
			t.Fatalf("%s: %v", c.op, err)
		}
		if res != s {
			t.Fatalf("%s rebound, want in place", c.op)
		}
		if got := sortedSetRepr(s); got != c.want {
			t.Fatalf("%s result = %s, want %s", c.op, got, c.want)
		}
	}
	// A non-set right operand declines to the binary op, which raises under the
	// augmented symbol.
	_, err := InPlace("-=", mk(1), NewList([]Object{NewInt(1)}))
	if err == nil || err.Error() != "TypeError: unsupported operand type(s) for -=: 'set' and 'list'" {
		t.Fatalf("set -= list error = %v", err)
	}
}

// A user in-place dunder returning self keeps the target object; returning
// NotImplemented falls through to the binary protocol.
func TestInPlaceUserDunder(t *testing.T) {
	c := mkclass(t, "Acc")
	c.setAttr("__iadd__", mkfn("Acc.__iadd__", 2, func(args []Object) (Object, error) {
		self := args[0].(*instanceObject)
		self.attrSet("v", args[1])
		return self, nil
	}))
	a := inst(c)
	res, err := InPlace("+=", a, NewInt(7))
	if err != nil {
		t.Fatalf("user += : %v", err)
	}
	if av, _ := a.attrGet("v"); res != a || Repr(av) != "7" {
		t.Fatalf("user += did not mutate self")
	}

	d := mkclass(t, "D")
	d.setAttr("__iadd__", mkfn("D.__iadd__", 2, func(args []Object) (Object, error) {
		return NotImplemented, nil
	}))
	d.setAttr("__add__", mkfn("D.__add__", 2, func(args []Object) (Object, error) {
		return NewStr("added"), nil
	}))
	x := inst(d)
	res, err = InPlace("+=", x, NewInt(1))
	if err != nil {
		t.Fatalf("NI fallback += : %v", err)
	}
	if res == Object(x) || Repr(res) != "'added'" {
		t.Fatalf("NI fallback returned %v", Repr(res))
	}
}

// An operand pair no handler accepts raises the unsupported-operand error under
// the augmented symbol, including ** which the binary form spells differently.
func TestInPlaceUnsupportedSymbol(t *testing.T) {
	g := inst(mkclass(t, "G"))
	for _, c := range []struct {
		op, want string
	}{
		{"+=", "unsupported operand type(s) for +=: 'G' and 'int'"},
		{"**=", "unsupported operand type(s) for **=: 'G' and 'int'"},
		{"|=", "unsupported operand type(s) for |=: 'G' and 'int'"},
	} {
		_, err := InPlace(c.op, g, NewInt(1))
		if err == nil || err.Error() != "TypeError: "+c.want {
			t.Fatalf("%s error = %v, want %s", c.op, err, c.want)
		}
	}
}

// Binary dict | builds a new dict keeping left order then the right's new keys,
// right winning on an overlap; both operands must be dicts.
func TestDictUnionBinary(t *testing.T) {
	a := D(t, NewInt(3), NewInt(1), NewInt(1), NewInt(2))
	b := D(t, NewInt(2), NewInt(3), NewInt(1), NewInt(9))
	res, err := BitOr(a, b)
	if err != nil {
		t.Fatalf("dict | dict: %v", err)
	}
	if got := Repr(res); got != "{3: 1, 1: 9, 2: 3}" {
		t.Fatalf("dict union = %s", got)
	}
	if res == Object(a) {
		t.Fatal("binary | must build a new dict, not mutate the left operand")
	}
	if got := Repr(a); got != "{3: 1, 1: 2}" {
		t.Fatalf("left operand changed to %s", got)
	}
	_, err = BitOr(D(t, NewInt(1), NewInt(2)), L(NewInt(1)))
	if err == nil || err.Error() != "TypeError: unsupported operand type(s) for |: 'dict' and 'list'" {
		t.Fatalf("dict | list error = %v", err)
	}
}

// dict |= merges in place through the update path, so it takes a mapping or an
// iterable of pairs and aliases see the merge.
func TestDictUnionInPlace(t *testing.T) {
	d := D(t, NewInt(1), NewStr("a"))
	res, err := InPlace("|=", d, D(t, NewInt(2), NewStr("b")))
	if err != nil {
		t.Fatalf("dict |= dict: %v", err)
	}
	if res != d || Repr(d) != `{1: 'a', 2: 'b'}` {
		t.Fatalf("|= dict = %s (same=%v)", Repr(d), res == d)
	}
	if _, err := InPlace("|=", d, L(T(NewInt(3), NewStr("c")))); err != nil {
		t.Fatalf("|= pairs: %v", err)
	}
	if got := Repr(d); got != `{1: 'a', 2: 'b', 3: 'c'}` {
		t.Fatalf("after |= pairs = %s", got)
	}
	_, err = InPlace("|=", D(t, NewInt(1), NewInt(2)), NewInt(5))
	if err == nil || err.Error() != "TypeError: 'int' object is not iterable" {
		t.Fatalf("|= int error = %v", err)
	}
}
