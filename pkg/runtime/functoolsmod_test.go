package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// ftFn loads a functools builtin through the module, the path compiled code
// takes for functools.reduce and functools.partial.
func ftFn(t *testing.T, name string) objects.Object {
	t.Helper()
	mo, err := ImportModule("functools")
	if err != nil {
		t.Fatalf("import functools: %v", err)
	}
	fn, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("functools.%s: %v", name, err)
	}
	return fn
}

// add2 is a two-argument adder used to drive reduce.
func add2(t *testing.T) objects.Object {
	t.Helper()
	return objects.NewFunc("add", 2, func(a []objects.Object) (objects.Object, error) {
		return objects.Add(a[0], a[1])
	})
}

func TestReduceFold(t *testing.T) {
	reduce := ftFn(t, "reduce")
	v, err := objects.Call(reduce, []objects.Object{add2(t), nums(1, 2, 3, 4)})
	if err != nil {
		t.Fatalf("reduce: %v", err)
	}
	if objects.Repr(v) != "10" {
		t.Fatalf("reduce fold = %s", objects.Repr(v))
	}
	v, _ = objects.Call(reduce, []objects.Object{add2(t), nums(1, 2, 3, 4), objects.NewInt(100)})
	if objects.Repr(v) != "110" {
		t.Fatalf("reduce with initial = %s", objects.Repr(v))
	}
	// An empty iterable seeds from the initializer, and returns it untouched.
	v, _ = objects.Call(reduce, []objects.Object{add2(t), nums(), objects.NewInt(42)})
	if objects.Repr(v) != "42" {
		t.Fatalf("reduce empty with initial = %s", objects.Repr(v))
	}
}

func TestReduceEmptyNoInitial(t *testing.T) {
	reduce := ftFn(t, "reduce")
	_, err := objects.Call(reduce, []objects.Object{add2(t), nums()})
	if err == nil || errText(err) != "reduce() of empty iterable with no initial value" {
		t.Fatalf("empty reduce error = %v", err)
	}
}

func TestReduceArity(t *testing.T) {
	reduce := ftFn(t, "reduce")
	_, err := objects.Call(reduce, []objects.Object{add2(t)})
	if err == nil || errText(err) != "reduce() takes at least 2 positional arguments (1 given)" {
		t.Fatalf("too-few error = %v", err)
	}
	_, err = objects.Call(reduce, []objects.Object{add2(t), nums(1), objects.NewInt(2), objects.NewInt(3)})
	if err == nil || errText(err) != "reduce() takes at most 3 arguments (4 given)" {
		t.Fatalf("too-many error = %v", err)
	}
}

// triple is a three-argument function returning its arguments as a tuple, used
// to observe how partial binds positionals and keywords.
func triple(t *testing.T) objects.Object {
	t.Helper()
	return objects.NewFunction("triple",
		[]objects.Param{
			{Name: "a", Kind: objects.ParamPlain},
			{Name: "b", Kind: objects.ParamPlain},
			{Name: "c", Kind: objects.ParamPlain},
		}, nil,
		func(a []objects.Object) (objects.Object, error) {
			return objects.NewTuple([]objects.Object{a[0], a[1], a[2]}), nil
		})
}

func TestPartialBindAndAttrs(t *testing.T) {
	partial := ftFn(t, "partial")
	f := triple(t)
	p, err := objects.CallKw(partial, []objects.Object{f, objects.NewInt(1)},
		[]string{"c"}, []objects.Object{objects.NewInt(3)})
	if err != nil {
		t.Fatalf("partial build: %v", err)
	}
	v, err := objects.Call(p, []objects.Object{objects.NewInt(2)})
	if err != nil {
		t.Fatalf("partial call: %v", err)
	}
	if objects.Repr(v) != "(1, 2, 3)" {
		t.Fatalf("partial call = %s", objects.Repr(v))
	}
	fn, _ := objects.LoadAttr(p, "func")
	if fn != f {
		t.Fatal("partial.func should be the wrapped callable")
	}
	args, _ := objects.LoadAttr(p, "args")
	if objects.Repr(args) != "(1,)" {
		t.Fatalf("partial.args = %s", objects.Repr(args))
	}
	kw, _ := objects.LoadAttr(p, "keywords")
	if objects.Repr(kw) != "{'c': 3}" {
		t.Fatalf("partial.keywords = %s", objects.Repr(kw))
	}
}

func TestPartialFlatten(t *testing.T) {
	partial := ftFn(t, "partial")
	f := triple(t)
	inner, _ := objects.Call(partial, []objects.Object{f, objects.NewInt(1)})
	outer, _ := objects.Call(partial, []objects.Object{inner, objects.NewInt(2)})
	// The outer partial folds into a single one over f, so func is f directly.
	fn, _ := objects.LoadAttr(outer, "func")
	if fn != f {
		t.Fatal("nested partial should flatten to the innermost callable")
	}
	v, _ := objects.Call(outer, []objects.Object{objects.NewInt(3)})
	if objects.Repr(v) != "(1, 2, 3)" {
		t.Fatalf("flattened call = %s", objects.Repr(v))
	}
}

func TestPartialRepr(t *testing.T) {
	partial := ftFn(t, "partial")
	// The len builtin reprs as a built-in function, a stable repr to anchor on.
	p, _ := objects.Call(partial, []objects.Object{BuiltinFn("len")})
	if got := objects.Repr(p); got != "functools.partial(<built-in function len>)" {
		t.Fatalf("partial repr = %q", got)
	}
}

func TestPartialErrors(t *testing.T) {
	partial := ftFn(t, "partial")
	_, err := objects.Call(partial, nil)
	if err == nil || errText(err) != "type 'partial' takes at least one argument" {
		t.Fatalf("empty partial error = %v", err)
	}
	_, err = objects.Call(partial, []objects.Object{objects.NewInt(1)})
	if err == nil || errText(err) != "the first argument must be callable" {
		t.Fatalf("non-callable error = %v", err)
	}
}

// counter returns a single-argument function that squares its input and records
// each call, so a test can watch the cache stop calls from reaching it.
func counter(seen *[]objects.Object) objects.Object {
	return objects.NewFunc("sq", 1, func(a []objects.Object) (objects.Object, error) {
		*seen = append(*seen, a[0])
		return objects.Mul(a[0], a[0])
	})
}

// info reads a field off the CacheInfo namedtuple cache_info returns.
func info(t *testing.T, wrapper objects.Object, field string) string {
	t.Helper()
	ci, err := objects.CallMethod(wrapper, "cache_info", nil)
	if err != nil {
		t.Fatalf("cache_info: %v", err)
	}
	v, err := objects.LoadAttr(ci, field)
	if err != nil {
		t.Fatalf("cache_info.%s: %v", field, err)
	}
	return objects.Repr(v)
}

func TestLRUCacheHitsMisses(t *testing.T) {
	var seen []objects.Object
	wrap, err := objects.CallKw(ftFn(t, "lru_cache"), nil,
		[]string{"maxsize"}, []objects.Object{objects.NewInt(2)})
	if err != nil {
		t.Fatalf("lru_cache(maxsize=2): %v", err)
	}
	sq, err := objects.Call(wrap, []objects.Object{counter(&seen)})
	if err != nil {
		t.Fatalf("decorate: %v", err)
	}
	for _, n := range []int64{2, 3, 2} {
		if _, err := objects.Call(sq, []objects.Object{objects.NewInt(n)}); err != nil {
			t.Fatalf("call sq(%d): %v", n, err)
		}
	}
	if got := info(t, sq, "hits"); got != "1" {
		t.Fatalf("hits = %s, want 1", got)
	}
	if got := info(t, sq, "misses"); got != "2" {
		t.Fatalf("misses = %s, want 2", got)
	}
	if got := info(t, sq, "currsize"); got != "2" {
		t.Fatalf("currsize = %s, want 2", got)
	}
	if len(seen) != 2 {
		t.Fatalf("underlying calls = %d, want 2", len(seen))
	}
}

func TestLRUCacheEviction(t *testing.T) {
	var seen []objects.Object
	wrap, _ := objects.CallKw(ftFn(t, "lru_cache"), nil,
		[]string{"maxsize"}, []objects.Object{objects.NewInt(2)})
	sq, _ := objects.Call(wrap, []objects.Object{counter(&seen)})
	// Fill with 2 and 3, touch 2 so 3 is the least recent, then 4 evicts 3.
	for _, n := range []int64{2, 3, 2, 4, 3} {
		objects.Call(sq, []objects.Object{objects.NewInt(n)})
	}
	if got := info(t, sq, "misses"); got != "4" {
		t.Fatalf("misses = %s, want 4 (3 recomputed after eviction)", got)
	}
	if got := info(t, sq, "currsize"); got != "2" {
		t.Fatalf("currsize = %s, want 2", got)
	}
}

func TestLRUCacheBareAndClear(t *testing.T) {
	var seen []objects.Object
	// Used bare, the first positional is the function and maxsize is 128.
	sq, err := objects.Call(ftFn(t, "lru_cache"), []objects.Object{counter(&seen)})
	if err != nil {
		t.Fatalf("bare lru_cache: %v", err)
	}
	objects.Call(sq, []objects.Object{objects.NewInt(5)})
	objects.Call(sq, []objects.Object{objects.NewInt(5)})
	if got := info(t, sq, "maxsize"); got != "128" {
		t.Fatalf("bare maxsize = %s, want 128", got)
	}
	if got := info(t, sq, "hits"); got != "1" {
		t.Fatalf("hits = %s, want 1", got)
	}
	if _, err := objects.CallMethod(sq, "cache_clear", nil); err != nil {
		t.Fatalf("cache_clear: %v", err)
	}
	if got := info(t, sq, "currsize"); got != "0" {
		t.Fatalf("currsize after clear = %s, want 0", got)
	}
	if got := info(t, sq, "hits"); got != "0" {
		t.Fatalf("hits after clear = %s, want 0", got)
	}
}

func TestLRUCacheTyped(t *testing.T) {
	var seen []objects.Object
	wrap, _ := objects.CallKw(ftFn(t, "lru_cache"), nil,
		[]string{"typed"}, []objects.Object{objects.True})
	f, _ := objects.Call(wrap, []objects.Object{counter(&seen)})
	// 3 and 3.0 are distinct keys when typed, so both compute; the repeated 3 hits.
	objects.Call(f, []objects.Object{objects.NewInt(3)})
	objects.Call(f, []objects.Object{objects.NewFloat(3.0)})
	objects.Call(f, []objects.Object{objects.NewInt(3)})
	if got := info(t, f, "misses"); got != "2" {
		t.Fatalf("typed misses = %s, want 2", got)
	}
	if got := info(t, f, "hits"); got != "1" {
		t.Fatalf("typed hits = %s, want 1", got)
	}
}

func TestCacheUnbounded(t *testing.T) {
	var seen []objects.Object
	f, err := objects.Call(ftFn(t, "cache"), []objects.Object{counter(&seen)})
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	objects.Call(f, []objects.Object{objects.NewInt(1)})
	objects.Call(f, []objects.Object{objects.NewInt(1)})
	if got := info(t, f, "maxsize"); got != "None" {
		t.Fatalf("cache maxsize = %s, want None", got)
	}
	if got := info(t, f, "hits"); got != "1" {
		t.Fatalf("cache hits = %s, want 1", got)
	}
}

func TestLRUCacheDisabled(t *testing.T) {
	var seen []objects.Object
	wrap, _ := objects.CallKw(ftFn(t, "lru_cache"), nil,
		[]string{"maxsize"}, []objects.Object{objects.NewInt(0)})
	f, _ := objects.Call(wrap, []objects.Object{counter(&seen)})
	objects.Call(f, []objects.Object{objects.NewInt(1)})
	objects.Call(f, []objects.Object{objects.NewInt(1)})
	if got := info(t, f, "misses"); got != "2" {
		t.Fatalf("disabled misses = %s, want 2", got)
	}
	if got := info(t, f, "currsize"); got != "0" {
		t.Fatalf("disabled currsize = %s, want 0", got)
	}
	if len(seen) != 2 {
		t.Fatalf("disabled underlying calls = %d, want 2", len(seen))
	}
}
