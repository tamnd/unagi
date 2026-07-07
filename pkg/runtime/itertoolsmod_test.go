package runtime

import (
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// itFn returns the named itertools function object.
func itFn(t *testing.T, name string) objects.Object {
	t.Helper()
	mo, err := ImportModule("itertools")
	if err != nil {
		t.Fatalf("import itertools: %v", err)
	}
	fn, err := objects.LoadAttr(mo, name)
	if err != nil {
		t.Fatalf("itertools.%s: %v", name, err)
	}
	return fn
}

// callIt calls itertools.<name>(args...) and returns the resulting iterator.
func callIt(t *testing.T, name string, args ...objects.Object) objects.Object {
	t.Helper()
	v, err := objects.Call(itFn(t, name), args)
	if err != nil {
		t.Fatalf("itertools.%s call: %v", name, err)
	}
	return v
}

// reprAll drains an iterator and joins the reprs, matching what str(ilist(...))
// would show element by element.
func reprAll(t *testing.T, o objects.Object) string {
	t.Helper()
	elts, err := materialize(o)
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	parts := make([]string, len(elts))
	for i, e := range elts {
		parts[i] = objects.Repr(e)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// takeN pulls the first n elements from an iterator, for the infinite ones.
func takeN(t *testing.T, o objects.Object, n int) string {
	t.Helper()
	it, err := objects.Iter(o)
	if err != nil {
		t.Fatalf("iter: %v", err)
	}
	parts := make([]string, 0, n)
	for range n {
		v, ok, err := it.Next()
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if !ok {
			break
		}
		parts = append(parts, objects.Repr(v))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func ints(ns ...int64) []objects.Object {
	out := make([]objects.Object, len(ns))
	for i, n := range ns {
		out[i] = objects.NewInt(n)
	}
	return out
}

func ilist(ns ...int64) objects.Object { return objects.NewList(ints(ns...)) }

func TestItertoolsInfinite(t *testing.T) {
	if got := takeN(t, callIt(t, "count"), 3); got != "[0, 1, 2]" {
		t.Errorf("count() = %s", got)
	}
	if got := takeN(t, callIt(t, "count", objects.NewInt(5), objects.NewInt(2)), 3); got != "[5, 7, 9]" {
		t.Errorf("count(5,2) = %s", got)
	}
	if got := takeN(t, callIt(t, "count", objects.NewFloat(2.5), objects.NewFloat(0.5)), 3); got != "[2.5, 3.0, 3.5]" {
		t.Errorf("count(2.5,0.5) = %s", got)
	}
	if got := takeN(t, callIt(t, "cycle", ilist(1, 2)), 5); got != "[1, 2, 1, 2, 1]" {
		t.Errorf("cycle([1,2]) = %s", got)
	}
	if got := takeN(t, callIt(t, "repeat", objects.NewInt(7)), 3); got != "[7, 7, 7]" {
		t.Errorf("repeat(7) = %s", got)
	}
	if got := reprAll(t, callIt(t, "repeat", objects.NewInt(7), objects.NewInt(2))); got != "[7, 7]" {
		t.Errorf("repeat(7,2) = %s", got)
	}
}

func TestItertoolsChain(t *testing.T) {
	if got := reprAll(t, callIt(t, "chain", ilist(1, 2), ilist(3), ilist(4, 5))); got != "[1, 2, 3, 4, 5]" {
		t.Errorf("chain = %s", got)
	}
	chain := itFn(t, "chain")
	fi, err := objects.LoadAttr(chain, "from_iterable")
	if err != nil {
		t.Fatalf("chain.from_iterable: %v", err)
	}
	outer := objects.NewList([]objects.Object{ilist(1, 2), ilist(3, 4)})
	v, err := objects.Call(fi, []objects.Object{outer})
	if err != nil {
		t.Fatalf("from_iterable call: %v", err)
	}
	if got := reprAll(t, v); got != "[1, 2, 3, 4]" {
		t.Errorf("chain.from_iterable = %s", got)
	}
}

func TestItertoolsIslice(t *testing.T) {
	rng := func() objects.Object { return ilist(0, 1, 2, 3, 4, 5, 6, 7, 8, 9) }
	if got := reprAll(t, callIt(t, "islice", rng(), objects.NewInt(3))); got != "[0, 1, 2]" {
		t.Errorf("islice(r,3) = %s", got)
	}
	if got := reprAll(t, callIt(t, "islice", rng(), objects.NewInt(2), objects.NewInt(8), objects.NewInt(2))); got != "[2, 4, 6]" {
		t.Errorf("islice(r,2,8,2) = %s", got)
	}
	_, err := objects.Call(itFn(t, "islice"), []objects.Object{rng(), objects.NewInt(-1)})
	if err == nil {
		t.Error("islice negative stop did not raise")
	} else if !strings.Contains(err.Error(), "0 <= x <= sys.maxsize") {
		t.Errorf("islice error = %v", err)
	}
}

func TestItertoolsSelectors(t *testing.T) {
	data := objects.NewList([]objects.Object{
		objects.NewStr("A"), objects.NewStr("B"), objects.NewStr("C"),
		objects.NewStr("D"), objects.NewStr("E"), objects.NewStr("F"),
	})
	sel := ilist(1, 0, 1, 0, 1, 1)
	if got := reprAll(t, callIt(t, "compress", data, sel)); got != "['A', 'C', 'E', 'F']" {
		t.Errorf("compress = %s", got)
	}

	// takewhile/dropwhile/filterfalse with a Python-level predicate via a lambda
	// are exercised in the fixture; here use the identity-truthiness forms.
	if got := reprAll(t, callIt(t, "filterfalse", objects.None, ilist(0, 1, 0, 2, 0))); got != "[0, 0, 0]" {
		t.Errorf("filterfalse(None) = %s", got)
	}
}

func TestItertoolsAccumulatePairwise(t *testing.T) {
	if got := reprAll(t, callIt(t, "accumulate", ilist(1, 2, 3, 4))); got != "[1, 3, 6, 10]" {
		t.Errorf("accumulate = %s", got)
	}
	acc, err := objects.CallKw(itFn(t, "accumulate"),
		[]objects.Object{ilist(1, 2, 3, 4)},
		[]string{"initial"}, []objects.Object{objects.NewInt(100)})
	if err != nil {
		t.Fatalf("accumulate initial: %v", err)
	}
	if got := reprAll(t, acc); got != "[100, 101, 103, 106, 110]" {
		t.Errorf("accumulate(initial=100) = %s", got)
	}
	if got := reprAll(t, callIt(t, "pairwise", ilist(1, 2, 3, 4))); got != "[(1, 2), (2, 3), (3, 4)]" {
		t.Errorf("pairwise = %s", got)
	}
}

func TestItertoolsZipLongest(t *testing.T) {
	z, err := objects.CallKw(itFn(t, "zip_longest"),
		[]objects.Object{ilist(1, 2, 3), ilist(4, 5)},
		[]string{"fillvalue"}, []objects.Object{objects.NewInt(0)})
	if err != nil {
		t.Fatalf("zip_longest: %v", err)
	}
	if got := reprAll(t, z); got != "[(1, 4), (2, 5), (3, 0)]" {
		t.Errorf("zip_longest = %s", got)
	}
}

func TestItertoolsProduct(t *testing.T) {
	if got := reprAll(t, callIt(t, "product", ilist(1, 2), ilist(3, 4))); got != "[(1, 3), (1, 4), (2, 3), (2, 4)]" {
		t.Errorf("product = %s", got)
	}
	p, err := objects.CallKw(itFn(t, "product"),
		[]objects.Object{ilist(1, 2)},
		[]string{"repeat"}, []objects.Object{objects.NewInt(2)})
	if err != nil {
		t.Fatalf("product repeat: %v", err)
	}
	if got := reprAll(t, p); got != "[(1, 1), (1, 2), (2, 1), (2, 2)]" {
		t.Errorf("product(repeat=2) = %s", got)
	}
}

func TestItertoolsCombinatorics(t *testing.T) {
	if got := reprAll(t, callIt(t, "permutations", ilist(1, 2, 3), objects.NewInt(2))); got != "[(1, 2), (1, 3), (2, 1), (2, 3), (3, 1), (3, 2)]" {
		t.Errorf("permutations = %s", got)
	}
	if got := reprAll(t, callIt(t, "combinations", ilist(1, 2, 3, 4), objects.NewInt(2))); got != "[(1, 2), (1, 3), (1, 4), (2, 3), (2, 4), (3, 4)]" {
		t.Errorf("combinations = %s", got)
	}
	if got := reprAll(t, callIt(t, "combinations_with_replacement", ilist(1, 2, 3), objects.NewInt(2))); got != "[(1, 1), (1, 2), (1, 3), (2, 2), (2, 3), (3, 3)]" {
		t.Errorf("combinations_with_replacement = %s", got)
	}
}

func TestItertoolsBatched(t *testing.T) {
	if got := reprAll(t, callIt(t, "batched", ilist(0, 1, 2, 3, 4), objects.NewInt(2))); got != "[(0, 1), (2, 3), (4,)]" {
		t.Errorf("batched = %s", got)
	}
	// Strict mode raises when the short final batch is pulled, not at
	// construction, so draining is what surfaces the error.
	bi, err := objects.CallKw(itFn(t, "batched"),
		[]objects.Object{ilist(0, 1, 2, 3, 4), objects.NewInt(2)},
		[]string{"strict"}, []objects.Object{objects.True})
	if err != nil {
		t.Fatalf("batched strict construct: %v", err)
	}
	if _, err := materialize(bi); err == nil {
		t.Fatal("batched strict did not raise on drain")
	} else if !strings.Contains(err.Error(), "incomplete batch") {
		t.Errorf("batched strict error = %v", err)
	}
}

func TestItertoolsGroupby(t *testing.T) {
	g := callIt(t, "groupby", ilist(1, 1, 2, 3, 3, 3, 1))
	it, err := objects.Iter(g)
	if err != nil {
		t.Fatal(err)
	}
	var groups []string
	for {
		pair, ok, err := it.Next()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		elts, _ := materialize(pair)
		groups = append(groups, reprAll(t, elts[1]))
	}
	got := strings.Join(groups, " ")
	if got != "[1, 1] [2] [3, 3, 3] [1]" {
		t.Errorf("groupby groups = %s", got)
	}
}

func TestItertoolsTee(t *testing.T) {
	tup, err := objects.Call(itFn(t, "tee"), []objects.Object{ilist(1, 2, 3)})
	if err != nil {
		t.Fatal(err)
	}
	readers, _ := materialize(tup)
	if len(readers) != 2 {
		t.Fatalf("tee gave %d readers", len(readers))
	}
	if got := reprAll(t, readers[0]); got != "[1, 2, 3]" {
		t.Errorf("tee[0] = %s", got)
	}
	if got := reprAll(t, readers[1]); got != "[1, 2, 3]" {
		t.Errorf("tee[1] = %s", got)
	}
}
