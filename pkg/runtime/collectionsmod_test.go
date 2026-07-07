package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// newDeque builds a deque through the collections module constructor, the same
// path compiled code takes for collections.deque(...).
func newDeque(t *testing.T, args ...objects.Object) objects.Object {
	t.Helper()
	mo, err := ImportModule("collections")
	if err != nil {
		t.Fatalf("import collections: %v", err)
	}
	fn, err := objects.LoadAttr(mo, "deque")
	if err != nil {
		t.Fatalf("collections.deque: %v", err)
	}
	d, err := objects.Call(fn, args)
	if err != nil {
		t.Fatalf("deque call: %v", err)
	}
	return d
}

// method calls d.name(args...) and fails on error.
func method(t *testing.T, d objects.Object, name string, args ...objects.Object) objects.Object {
	t.Helper()
	v, err := objects.CallMethod(d, name, args)
	if err != nil {
		t.Fatalf("%s.%s: %v", d.TypeName(), name, err)
	}
	return v
}

func nums(vals ...int64) objects.Object {
	elts := make([]objects.Object, len(vals))
	for i, v := range vals {
		elts[i] = objects.NewInt(v)
	}
	return objects.NewList(elts)
}

func TestDequeConstructAndRepr(t *testing.T) {
	d := newDeque(t)
	if got := objects.Repr(d); got != "deque([])" {
		t.Fatalf("empty deque repr = %q", got)
	}
	d = newDeque(t, nums(1, 2, 3))
	if got := objects.Repr(d); got != "deque([1, 2, 3])" {
		t.Fatalf("deque repr = %q", got)
	}
	d = newDeque(t, nums(1, 2, 3, 4, 5), objects.NewInt(3))
	if got := objects.Repr(d); got != "deque([3, 4, 5], maxlen=3)" {
		t.Fatalf("bounded deque repr = %q", got)
	}
}

func TestDequeAppendAndPop(t *testing.T) {
	d := newDeque(t, nums(1, 2, 3))
	method(t, d, "append", objects.NewInt(4))
	method(t, d, "appendleft", objects.NewInt(0))
	if got := objects.Repr(d); got != "deque([0, 1, 2, 3, 4])" {
		t.Fatalf("after appends = %q", got)
	}
	if v := method(t, d, "pop"); objects.Repr(v) != "4" {
		t.Fatalf("pop = %s", objects.Repr(v))
	}
	if v := method(t, d, "popleft"); objects.Repr(v) != "0" {
		t.Fatalf("popleft = %s", objects.Repr(v))
	}
	if got := objects.Repr(d); got != "deque([1, 2, 3])" {
		t.Fatalf("after pops = %q", got)
	}
}

func TestDequePopEmpty(t *testing.T) {
	d := newDeque(t)
	if _, err := objects.CallMethod(d, "pop", nil); err == nil {
		t.Fatal("pop from empty should raise")
	}
}

func TestDequeMaxlenEviction(t *testing.T) {
	d := newDeque(t, nums(), objects.NewInt(2))
	method(t, d, "append", objects.NewInt(1))
	method(t, d, "append", objects.NewInt(2))
	method(t, d, "append", objects.NewInt(3))
	if got := objects.Repr(d); got != "deque([2, 3], maxlen=2)" {
		t.Fatalf("append eviction = %q", got)
	}
	method(t, d, "appendleft", objects.NewInt(0))
	if got := objects.Repr(d); got != "deque([0, 2], maxlen=2)" {
		t.Fatalf("appendleft eviction = %q", got)
	}
}

func TestDequeMaxlenAttr(t *testing.T) {
	d := newDeque(t)
	if v, _ := objects.LoadAttr(d, "maxlen"); v != objects.None {
		t.Fatalf("unbounded maxlen = %s", objects.Repr(v))
	}
	d = newDeque(t, nums(), objects.NewInt(5))
	if v, _ := objects.LoadAttr(d, "maxlen"); objects.Repr(v) != "5" {
		t.Fatalf("bounded maxlen = %s", objects.Repr(v))
	}
}

func TestDequeMaxlenNegative(t *testing.T) {
	mo, _ := ImportModule("collections")
	fn, _ := objects.LoadAttr(mo, "deque")
	if _, err := objects.Call(fn, []objects.Object{objects.None, objects.NewInt(-1)}); err == nil {
		t.Fatal("negative maxlen should raise")
	}
}

func TestDequeRotate(t *testing.T) {
	d := newDeque(t, nums(1, 2, 3, 4, 5))
	method(t, d, "rotate", objects.NewInt(2))
	if got := objects.Repr(d); got != "deque([4, 5, 1, 2, 3])" {
		t.Fatalf("rotate right = %q", got)
	}
	method(t, d, "rotate", objects.NewInt(-2))
	if got := objects.Repr(d); got != "deque([1, 2, 3, 4, 5])" {
		t.Fatalf("rotate left = %q", got)
	}
}

func TestDequeExtend(t *testing.T) {
	d := newDeque(t, nums(1, 2))
	method(t, d, "extend", nums(3, 4))
	method(t, d, "extendleft", nums(0, -1))
	if got := objects.Repr(d); got != "deque([-1, 0, 1, 2, 3, 4])" {
		t.Fatalf("extend = %q", got)
	}
}

func TestDequeIndexRemoveCount(t *testing.T) {
	d := newDeque(t, nums(1, 2, 3, 2, 1))
	if v := method(t, d, "index", objects.NewInt(2)); objects.Repr(v) != "1" {
		t.Fatalf("index = %s", objects.Repr(v))
	}
	if v := method(t, d, "count", objects.NewInt(1)); objects.Repr(v) != "2" {
		t.Fatalf("count = %s", objects.Repr(v))
	}
	method(t, d, "remove", objects.NewInt(2))
	if got := objects.Repr(d); got != "deque([1, 3, 2, 1])" {
		t.Fatalf("after remove = %q", got)
	}
}

func TestDequeSubscript(t *testing.T) {
	d := newDeque(t, nums(10, 20, 30))
	v, err := objects.GetItem(d, objects.NewInt(1))
	if err != nil || objects.Repr(v) != "20" {
		t.Fatalf("d[1] = %s err %v", objects.Repr(v), err)
	}
	if err := objects.SetItem(d, objects.NewInt(0), objects.NewInt(99)); err != nil {
		t.Fatalf("setitem: %v", err)
	}
	v, _ = objects.GetItem(d, objects.NewInt(-1))
	if objects.Repr(v) != "30" {
		t.Fatalf("d[-1] = %s", objects.Repr(v))
	}
	if got := objects.Repr(d); got != "deque([99, 20, 30])" {
		t.Fatalf("after setitem = %q", got)
	}
}

// collFn returns a named callable from the collections module.
func collFn(t *testing.T, name string) objects.Object {
	t.Helper()
	mo, err := ImportModule("collections")
	if err != nil {
		t.Fatalf("import collections: %v", err)
	}
	fn, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("collections.%s: %v", name, err)
	}
	return fn
}

// builtin returns a global builtin callable such as list or int, the factory a
// defaultdict is usually built with.
func builtin(t *testing.T, name string) objects.Object {
	t.Helper()
	v, ok := builtins[name]
	if !ok {
		t.Fatalf("builtin %s not registered", name)
	}
	return v
}

func TestDefaultDictFactoryFill(t *testing.T) {
	dd, err := objects.Call(collFn(t, "defaultdict"), []objects.Object{builtin(t, "list")})
	if err != nil {
		t.Fatalf("defaultdict(list): %v", err)
	}
	// d['a'] on a missing key calls list() and stores it.
	v, err := objects.GetItem(dd, objects.NewStr("a"))
	if err != nil {
		t.Fatalf("d['a']: %v", err)
	}
	if _, err := objects.CallMethod(v, "append", []objects.Object{objects.NewInt(1)}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if got := objects.Repr(dd); got != "defaultdict(<class 'list'>, {'a': [1]})" {
		t.Fatalf("defaultdict repr = %q", got)
	}
}

func TestDefaultDictNoneFactory(t *testing.T) {
	dd, err := objects.Call(collFn(t, "defaultdict"), nil)
	if err != nil {
		t.Fatalf("defaultdict(): %v", err)
	}
	if v, _ := objects.LoadAttr(dd, "default_factory"); v != objects.None {
		t.Fatalf("default_factory = %s", objects.Repr(v))
	}
	// A None factory raises KeyError on a missing key like a plain dict.
	if _, err := objects.GetItem(dd, objects.NewStr("x")); err == nil {
		t.Fatal("missing key with None factory should raise KeyError")
	}
	if got := objects.Repr(dd); got != "defaultdict(None, {})" {
		t.Fatalf("empty defaultdict repr = %q", got)
	}
}

func TestDefaultDictSeededAndEqual(t *testing.T) {
	seed, _ := objects.NewDict([]objects.Object{objects.NewStr("x")}, []objects.Object{objects.NewInt(5)})
	dd, err := objects.Call(collFn(t, "defaultdict"), []objects.Object{builtin(t, "int"), seed})
	if err != nil {
		t.Fatalf("defaultdict(int, {...}): %v", err)
	}
	v, _ := objects.GetItem(dd, objects.NewStr("x"))
	if objects.Repr(v) != "5" {
		t.Fatalf("seeded value = %s", objects.Repr(v))
	}
	// A missing key fills with int() == 0.
	v, _ = objects.GetItem(dd, objects.NewStr("y"))
	if objects.Repr(v) != "0" {
		t.Fatalf("filled value = %s", objects.Repr(v))
	}
	// A defaultdict equals a plain dict with the same items.
	plain, _ := objects.NewDict(
		[]objects.Object{objects.NewStr("x"), objects.NewStr("y")},
		[]objects.Object{objects.NewInt(5), objects.NewInt(0)})
	res, _ := objects.Compare(objects.OpEq, dd, plain)
	if !objects.Truth(res) {
		t.Fatal("defaultdict should equal a plain dict with the same items")
	}
}

func TestDefaultDictBadFactory(t *testing.T) {
	if _, err := objects.Call(collFn(t, "defaultdict"), []objects.Object{objects.NewInt(5)}); err == nil {
		t.Fatal("non-callable factory should raise TypeError")
	}
}

func TestDefaultDictSetFactory(t *testing.T) {
	dd, _ := objects.Call(collFn(t, "defaultdict"), nil)
	if err := objects.StoreAttr(dd, "default_factory", builtin(t, "int")); err != nil {
		t.Fatalf("set default_factory: %v", err)
	}
	v, _ := objects.GetItem(dd, objects.NewStr("z"))
	if objects.Repr(v) != "0" {
		t.Fatalf("after setting factory, d['z'] = %s", objects.Repr(v))
	}
}

func TestDefaultDictCopy(t *testing.T) {
	dd, _ := objects.Call(collFn(t, "defaultdict"), []objects.Object{builtin(t, "list")})
	c, err := objects.CallMethod(dd, "copy", nil)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if c.TypeName() != "collections.defaultdict" {
		t.Fatalf("copy type = %s", c.TypeName())
	}
	// The copy keeps the factory, so a missing key still fills.
	if _, err := objects.GetItem(c, objects.NewStr("q")); err != nil {
		t.Fatalf("copy fill: %v", err)
	}
}

// newCounter builds a Counter through the collections module constructor.
func newCounter(t *testing.T, args ...objects.Object) objects.Object {
	t.Helper()
	c, err := objects.Call(collFn(t, "Counter"), args)
	if err != nil {
		t.Fatalf("Counter call: %v", err)
	}
	return c
}

func TestCounterCountAndRepr(t *testing.T) {
	c := newCounter(t, objects.NewStr("mississippi"))
	if got := objects.Repr(c); got != "Counter({'i': 4, 's': 4, 'p': 2, 'm': 1})" {
		t.Fatalf("counter repr = %q", got)
	}
	// A missing element reads zero without growing the mapping.
	v, err := objects.GetItem(c, objects.NewStr("z"))
	if err != nil || objects.Repr(v) != "0" {
		t.Fatalf("missing element = %s err %v", objects.Repr(v), err)
	}
	if n, _ := objects.Len(c); n != 4 {
		t.Fatalf("len after missing read = %d", n)
	}
	if got := objects.Repr(newCounter(t)); got != "Counter()" {
		t.Fatalf("empty counter repr = %q", got)
	}
}

func TestCounterMostCommonAndElements(t *testing.T) {
	c := newCounter(t, objects.NewStr("mississippi"))
	mc := method(t, c, "most_common", objects.NewInt(2))
	if got := objects.Repr(mc); got != "[('i', 4), ('s', 4)]" {
		t.Fatalf("most_common(2) = %q", got)
	}
	elts := builtinList(t, method(t, c, "elements"))
	method(t, elts, "sort")
	if got := objects.Repr(elts); got != "['i', 'i', 'i', 'i', 'm', 'p', 'p', 's', 's', 's', 's']" {
		t.Fatalf("sorted elements = %q", got)
	}
}

func TestCounterArithmetic(t *testing.T) {
	mk := func(a, b int) objects.Object {
		return newCounter(t, dictOf(t, "a", a, "b", b))
	}
	cases := []struct {
		op   func(x, y objects.Object) (objects.Object, error)
		want string
	}{
		{objects.Add, "Counter({'a': 4, 'b': 3})"},
		{objects.Sub, "Counter({'a': 2})"},
		{objects.BitAnd, "Counter({'a': 1, 'b': 1})"},
		{objects.BitOr, "Counter({'a': 3, 'b': 2})"},
	}
	for _, tc := range cases {
		v, err := tc.op(mk(3, 1), mk(1, 2))
		if err != nil {
			t.Fatalf("op: %v", err)
		}
		if got := objects.Repr(v); got != tc.want {
			t.Fatalf("op result = %q want %q", got, tc.want)
		}
	}
}

func TestCounterUnary(t *testing.T) {
	c := newCounter(t, dictOf(t, "a", 1, "b", -1))
	pos, _ := objects.Pos(c)
	if got := objects.Repr(pos); got != "Counter({'a': 1})" {
		t.Fatalf("+counter = %q", got)
	}
	neg, _ := objects.Neg(c)
	if got := objects.Repr(neg); got != "Counter({'b': 1})" {
		t.Fatalf("-counter = %q", got)
	}
}

func TestCounterUpdateSubtract(t *testing.T) {
	c := newCounter(t, objects.NewStr("aab"))
	method(t, c, "subtract", objects.NewStr("ab"))
	if got := objects.Repr(c); got != "Counter({'a': 1, 'b': 0})" {
		t.Fatalf("after subtract = %q", got)
	}
	method(t, c, "update", nums())
	method(t, c, "update", objects.NewList([]objects.Object{objects.NewStr("a"), objects.NewStr("x")}))
	if got := objects.Repr(c); got != "Counter({'a': 2, 'x': 1, 'b': 0})" {
		t.Fatalf("after update = %q", got)
	}
}

func TestCounterEqualsDictAndTotal(t *testing.T) {
	c := newCounter(t, dictOf(t, "a", 1))
	plain, _ := objects.NewDict([]objects.Object{objects.NewStr("a")}, []objects.Object{objects.NewInt(1)})
	res, _ := objects.Compare(objects.OpEq, c, plain)
	if !objects.Truth(res) {
		t.Fatal("Counter should equal a plain dict with the same items")
	}
	c2 := newCounter(t, dictOf(t, "x", 5, "y", 2))
	if got := objects.Repr(method(t, c2, "total")); got != "7" {
		t.Fatalf("total = %s", got)
	}
	// Counter | plain dict falls back to the dict union, a plain dict.
	u, err := objects.BitOr(newCounter(t, dictOf(t, "a", 1)), plain)
	if err != nil {
		t.Fatalf("counter | dict: %v", err)
	}
	if got := objects.Repr(u); got != "{'a': 1}" || u.TypeName() != "dict" {
		t.Fatalf("counter | dict = %q type %s", got, u.TypeName())
	}
}

// dictOf builds a plain dict from alternating string keys and int values, a
// compact way to seed a Counter in the tests.
func dictOf(t *testing.T, kv ...any) objects.Object {
	t.Helper()
	var keys, vals []objects.Object
	for i := 0; i < len(kv); i += 2 {
		keys = append(keys, objects.NewStr(kv[i].(string)))
		vals = append(vals, objects.NewInt(int64(kv[i+1].(int))))
	}
	d, err := objects.NewDict(keys, vals)
	if err != nil {
		t.Fatalf("NewDict: %v", err)
	}
	return d
}

// builtinList materializes an iterable into a list so the test can sort it.
func builtinList(t *testing.T, it objects.Object) objects.Object {
	t.Helper()
	elts, err := materialize(it)
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	return objects.NewList(elts)
}

func TestDequeEqualityAndLen(t *testing.T) {
	a := newDeque(t, nums(1, 2, 3))
	b := newDeque(t, nums(1, 2, 3))
	if !objects.Truth(a) {
		t.Fatal("non-empty deque should be truthy")
	}
	n, _ := objects.Len(a)
	if n != 3 {
		t.Fatalf("len = %d", n)
	}
	res, _ := objects.Compare(objects.OpEq, a, b)
	if !objects.Truth(res) {
		t.Fatal("equal deques should compare equal")
	}
	// A deque never equals a list with the same contents.
	res, _ = objects.Compare(objects.OpEq, a, nums(1, 2, 3))
	if objects.Truth(res) {
		t.Fatal("deque should not equal a list")
	}
}
