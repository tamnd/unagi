package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// itertools is a built-in module: CPython ships it in C with no pure-Python
// fallback, so the runtime owns the public names with hand-written lazy iterator
// objects. Each object pulls from its sources one element at a time and returns
// itself from iter(), so next() and a for loop share one cursor and any side
// effect in a supplied function is observable as the result is consumed, the way
// CPython's iterators behave.

func init() {
	moduleTable["itertools"] = &moduleEntry{builtin: true, exec: initItertools}
}

func initItertools(m *objects.Module) error {
	set := func(name string, v objects.Object) error { return objects.StoreAttr(m, name, v) }

	count := objects.NewFunction("count",
		[]objects.Param{{Name: "start", Kind: objects.ParamPlain}, {Name: "step", Kind: objects.ParamPlain}},
		[]objects.Object{objects.NewInt(0), objects.NewInt(1)},
		func(a []objects.Object) (objects.Object, error) {
			return &countIter{cur: a[0], step: a[1]}, nil
		})
	if err := set("count", count); err != nil {
		return err
	}

	if err := set("cycle", objects.NewFunc("cycle", 1, func(a []objects.Object) (objects.Object, error) {
		it, err := objects.Iter(a[0])
		if err != nil {
			return nil, err
		}
		return &cycleIter{it: it}, nil
	})); err != nil {
		return err
	}

	repeat := objects.NewFunction("repeat",
		[]objects.Param{{Name: "object", Kind: objects.ParamPlain}, {Name: "times", Kind: objects.ParamPlain}},
		[]objects.Object{nil, objects.None},
		func(a []objects.Object) (objects.Object, error) {
			r := &repeatIter{obj: a[0], infinite: true}
			if a[1] != objects.None {
				n, err := asIndex(a[1])
				if err != nil {
					return nil, err
				}
				r.infinite = false
				if n > 0 {
					r.left = int(n)
				}
			}
			return r, nil
		})
	if err := set("repeat", repeat); err != nil {
		return err
	}

	chain := objects.NewFunc("chain", -1, func(a []objects.Object) (objects.Object, error) {
		return &chainIter{srcs: append([]objects.Object{}, a...)}, nil
	})
	fromIterable := objects.NewFunc("chain.from_iterable", 1, func(a []objects.Object) (objects.Object, error) {
		outer, err := objects.Iter(a[0])
		if err != nil {
			return nil, err
		}
		return &chainIter{outer: outer, fromOuter: true}, nil
	})
	if err := objects.SetBuiltinAttr(chain, "from_iterable", fromIterable); err != nil {
		return err
	}
	if err := set("chain", chain); err != nil {
		return err
	}

	if err := set("islice", objects.NewFunc("islice", -1, itertoolsIslice)); err != nil {
		return err
	}

	if err := set("compress", objects.NewFunc("compress", 2, func(a []objects.Object) (objects.Object, error) {
		data, err := objects.Iter(a[0])
		if err != nil {
			return nil, err
		}
		sel, err := objects.Iter(a[1])
		if err != nil {
			return nil, err
		}
		return &compressIter{data: data, sel: sel}, nil
	})); err != nil {
		return err
	}

	for _, w := range []struct {
		name string
		drop bool
	}{{"takewhile", false}, {"dropwhile", true}} {
		if err := set(w.name, objects.NewFunc(w.name, 2, func(a []objects.Object) (objects.Object, error) {
			it, err := objects.Iter(a[1])
			if err != nil {
				return nil, err
			}
			return &whileIter{pred: a[0], it: it, drop: w.drop}, nil
		})); err != nil {
			return err
		}
	}

	if err := set("filterfalse", objects.NewFunc("filterfalse", 2, func(a []objects.Object) (objects.Object, error) {
		it, err := objects.Iter(a[1])
		if err != nil {
			return nil, err
		}
		pred := a[0]
		if pred == objects.None {
			pred = nil
		}
		return &filterfalseIter{pred: pred, it: it}, nil
	})); err != nil {
		return err
	}

	if err := set("starmap", objects.NewFunc("starmap", 2, func(a []objects.Object) (objects.Object, error) {
		it, err := objects.Iter(a[1])
		if err != nil {
			return nil, err
		}
		return &starmapIter{fn: a[0], it: it}, nil
	})); err != nil {
		return err
	}

	accumulate := objects.NewFunction("accumulate",
		[]objects.Param{
			{Name: "iterable", Kind: objects.ParamPlain},
			{Name: "func", Kind: objects.ParamPlain},
			{Name: "initial", Kind: objects.ParamKwOnly},
		},
		[]objects.Object{nil, objects.None, objects.None},
		func(a []objects.Object) (objects.Object, error) {
			it, err := objects.Iter(a[0])
			if err != nil {
				return nil, err
			}
			acc := &accumulateIter{it: it}
			if a[1] != objects.None {
				acc.fn = a[1]
			}
			if a[2] != objects.None {
				acc.total = a[2]
				acc.hasTotal = true
			}
			return acc, nil
		})
	if err := set("accumulate", accumulate); err != nil {
		return err
	}

	if err := set("pairwise", objects.NewFunc("pairwise", 1, func(a []objects.Object) (objects.Object, error) {
		it, err := objects.Iter(a[0])
		if err != nil {
			return nil, err
		}
		return &pairwiseIter{it: it}, nil
	})); err != nil {
		return err
	}

	zipLongest := objects.NewFunction("zip_longest",
		[]objects.Param{{Name: "iterables", Kind: objects.ParamStar}, {Name: "fillvalue", Kind: objects.ParamKwOnly}},
		[]objects.Object{nil, objects.None},
		func(a []objects.Object) (objects.Object, error) {
			srcs := tupleElems(a[0])
			iters := make([]objects.Iterator, len(srcs))
			for i, s := range srcs {
				it, err := objects.Iter(s)
				if err != nil {
					return nil, err
				}
				iters[i] = it
			}
			return &zipLongestIter{iters: iters, fill: a[1], live: len(iters)}, nil
		})
	if err := set("zip_longest", zipLongest); err != nil {
		return err
	}

	if err := set("tee", objects.NewFunc("tee", -1, itertoolsTee)); err != nil {
		return err
	}

	groupby := objects.NewFunction("groupby",
		[]objects.Param{{Name: "iterable", Kind: objects.ParamPlain}, {Name: "key", Kind: objects.ParamPlain}},
		[]objects.Object{nil, objects.None},
		func(a []objects.Object) (objects.Object, error) {
			it, err := objects.Iter(a[0])
			if err != nil {
				return nil, err
			}
			g := &groupbyIter{it: it}
			if a[1] != objects.None {
				g.keyfn = a[1]
			}
			return g, nil
		})
	if err := set("groupby", groupby); err != nil {
		return err
	}

	batched := objects.NewFunction("batched",
		[]objects.Param{
			{Name: "iterable", Kind: objects.ParamPlain},
			{Name: "n", Kind: objects.ParamPlain},
			{Name: "strict", Kind: objects.ParamKwOnly},
		},
		[]objects.Object{nil, nil, objects.False},
		func(a []objects.Object) (objects.Object, error) {
			n, err := asIndex(a[1])
			if err != nil {
				return nil, err
			}
			if n < 1 {
				return nil, objects.Raise(objects.ValueError, "n must be at least one")
			}
			it, err := objects.Iter(a[0])
			if err != nil {
				return nil, err
			}
			return &batchedIter{it: it, n: int(n), strict: objects.Truth(a[2])}, nil
		})
	if err := set("batched", batched); err != nil {
		return err
	}

	product := objects.NewFunction("product",
		[]objects.Param{{Name: "iterables", Kind: objects.ParamStar}, {Name: "repeat", Kind: objects.ParamKwOnly}},
		[]objects.Object{nil, objects.NewInt(1)},
		itertoolsProduct)
	if err := set("product", product); err != nil {
		return err
	}

	permutations := objects.NewFunction("permutations",
		[]objects.Param{{Name: "iterable", Kind: objects.ParamPlain}, {Name: "r", Kind: objects.ParamPlain}},
		[]objects.Object{nil, objects.None},
		itertoolsPermutations)
	if err := set("permutations", permutations); err != nil {
		return err
	}

	if err := set("combinations", objects.NewFunc("combinations", 2, func(a []objects.Object) (objects.Object, error) {
		return itertoolsCombinations(a[0], a[1], false)
	})); err != nil {
		return err
	}
	if err := set("combinations_with_replacement", objects.NewFunc("combinations_with_replacement", 2,
		func(a []objects.Object) (objects.Object, error) {
			return itertoolsCombinations(a[0], a[1], true)
		})); err != nil {
		return err
	}

	return nil
}

// tupleElems returns the elements of the tuple a ParamStar slot carries. The
// slot is always a tuple, so draining it never fails on a well-formed call.
func tupleElems(o objects.Object) []objects.Object {
	elts, _ := materialize(o)
	return elts
}

// countIter yields start, start+step, start+2*step, ... forever, preserving the
// int or float type of the running value the way itertools.count does.
type countIter struct {
	cur  objects.Object
	step objects.Object
}

func (c *countIter) TypeName() string                   { return "count" }
func (c *countIter) Iterate() (objects.Iterator, error) { return c, nil }

func (c *countIter) Next() (objects.Object, bool, error) {
	v := c.cur
	next, err := objects.Add(c.cur, c.step)
	if err != nil {
		return nil, false, err
	}
	c.cur = next
	return v, true, nil
}

// cycleIter yields the source once while remembering every element, then repeats
// the saved elements forever. An empty source yields nothing.
type cycleIter struct {
	it    objects.Iterator
	saved []objects.Object
	i     int
	done  bool
}

func (c *cycleIter) TypeName() string                   { return "cycle" }
func (c *cycleIter) Iterate() (objects.Iterator, error) { return c, nil }

func (c *cycleIter) Next() (objects.Object, bool, error) {
	if !c.done {
		v, ok, err := c.it.Next()
		if err != nil {
			return nil, false, err
		}
		if ok {
			c.saved = append(c.saved, v)
			return v, true, nil
		}
		c.done = true
	}
	if len(c.saved) == 0 {
		return nil, false, nil
	}
	v := c.saved[c.i]
	c.i = (c.i + 1) % len(c.saved)
	return v, true, nil
}

// repeatIter yields obj a fixed number of times, or forever when times is None.
type repeatIter struct {
	obj      objects.Object
	left     int
	infinite bool
}

func (r *repeatIter) TypeName() string                   { return "repeat" }
func (r *repeatIter) Iterate() (objects.Iterator, error) { return r, nil }

func (r *repeatIter) Next() (objects.Object, bool, error) {
	if r.infinite {
		return r.obj, true, nil
	}
	if r.left <= 0 {
		return nil, false, nil
	}
	r.left--
	return r.obj, true, nil
}

// chainIter yields the elements of each source in turn. A fixed list of sources
// backs chain(*iterables); an outer iterator of iterables backs
// chain.from_iterable.
type chainIter struct {
	srcs      []objects.Object
	idx       int
	outer     objects.Iterator
	fromOuter bool
	cur       objects.Iterator
}

func (c *chainIter) TypeName() string                   { return "chain" }
func (c *chainIter) Iterate() (objects.Iterator, error) { return c, nil }

func (c *chainIter) Next() (objects.Object, bool, error) {
	for {
		if c.cur == nil {
			src, ok, err := c.nextSource()
			if err != nil {
				return nil, false, err
			}
			if !ok {
				return nil, false, nil
			}
			it, err := objects.Iter(src)
			if err != nil {
				return nil, false, err
			}
			c.cur = it
		}
		v, ok, err := c.cur.Next()
		if err != nil {
			return nil, false, err
		}
		if !ok {
			c.cur = nil
			continue
		}
		return v, true, nil
	}
}

func (c *chainIter) nextSource() (objects.Object, bool, error) {
	if c.fromOuter {
		return c.outer.Next()
	}
	if c.idx >= len(c.srcs) {
		return nil, false, nil
	}
	src := c.srcs[c.idx]
	c.idx++
	return src, true, nil
}

// itertoolsIslice implements islice(iterable, stop) and
// islice(iterable, start, stop[, step]).
func itertoolsIslice(a []objects.Object) (objects.Object, error) {
	if len(a) < 2 || len(a) > 4 {
		return nil, objects.Raise(objects.TypeError,
			"islice expected 2 to 4 arguments, got %d", len(a))
	}
	it, err := objects.Iter(a[0])
	if err != nil {
		return nil, err
	}
	sl := &isliceIter{it: it, step: 1}
	sliceInt := func(o objects.Object, what string) (int, bool, error) {
		if o == objects.None {
			return 0, false, nil
		}
		n, err := asIndex(o)
		if err != nil || n < 0 {
			return 0, false, objects.Raise(objects.ValueError,
				"%s argument for islice() must be None or an integer: 0 <= x <= sys.maxsize.", what)
		}
		return int(n), true, nil
	}
	if len(a) == 2 {
		stop, has, err := sliceInt(a[1], "Stop")
		if err != nil {
			return nil, err
		}
		sl.stop, sl.hasStop = stop, has
	} else {
		start, _, err := sliceInt(a[1], "Start")
		if err != nil {
			return nil, err
		}
		stop, has, err := sliceInt(a[2], "Stop")
		if err != nil {
			return nil, err
		}
		sl.next, sl.stop, sl.hasStop = start, stop, has
		if len(a) == 4 && a[3] != objects.None {
			step, err := asIndex(a[3])
			if err != nil || step < 1 {
				return nil, objects.Raise(objects.ValueError,
					"Step for islice() must be a positive integer or None.")
			}
			sl.step = int(step)
		}
	}
	return sl, nil
}

// isliceIter walks the source counting positions, yielding the element at next,
// next+step, ... until the stop bound.
type isliceIter struct {
	it      objects.Iterator
	count   int
	next    int
	stop    int
	hasStop bool
	step    int
}

func (s *isliceIter) TypeName() string                   { return "islice" }
func (s *isliceIter) Iterate() (objects.Iterator, error) { return s, nil }

func (s *isliceIter) Next() (objects.Object, bool, error) {
	for {
		if s.hasStop && s.count >= s.stop {
			return nil, false, nil
		}
		v, ok, err := s.it.Next()
		if err != nil {
			return nil, false, err
		}
		if !ok {
			return nil, false, nil
		}
		idx := s.count
		s.count++
		if idx == s.next {
			s.next += s.step
			return v, true, nil
		}
	}
}

// compressIter yields the data elements whose matching selector is truthy,
// stopping when either input runs out.
type compressIter struct {
	data objects.Iterator
	sel  objects.Iterator
}

func (c *compressIter) TypeName() string                   { return "compress" }
func (c *compressIter) Iterate() (objects.Iterator, error) { return c, nil }

func (c *compressIter) Next() (objects.Object, bool, error) {
	for {
		d, ok, err := c.data.Next()
		if err != nil || !ok {
			return nil, false, err
		}
		s, ok, err := c.sel.Next()
		if err != nil || !ok {
			return nil, false, err
		}
		keep, err := objects.TruthOf(s)
		if err != nil {
			return nil, false, err
		}
		if keep {
			return d, true, nil
		}
	}
}

// whileIter is takewhile and dropwhile: takewhile yields the leading run for
// which the predicate holds and then stops; dropwhile discards that run and
// yields everything after it.
type whileIter struct {
	pred    objects.Object
	it      objects.Iterator
	drop    bool
	dropped bool
	stopped bool
}

func (w *whileIter) TypeName() string {
	if w.drop {
		return "dropwhile"
	}
	return "takewhile"
}
func (w *whileIter) Iterate() (objects.Iterator, error) { return w, nil }

func (w *whileIter) Next() (objects.Object, bool, error) {
	if w.stopped {
		return nil, false, nil
	}
	if w.drop {
		for !w.dropped {
			v, ok, err := w.it.Next()
			if err != nil || !ok {
				return nil, false, err
			}
			t, err := w.test(v)
			if err != nil {
				return nil, false, err
			}
			if !t {
				w.dropped = true
				return v, true, nil
			}
		}
		return w.it.Next()
	}
	v, ok, err := w.it.Next()
	if err != nil || !ok {
		return nil, false, err
	}
	t, err := w.test(v)
	if err != nil {
		return nil, false, err
	}
	if !t {
		w.stopped = true
		return nil, false, nil
	}
	return v, true, nil
}

func (w *whileIter) test(v objects.Object) (bool, error) {
	r, err := objects.Call(w.pred, []objects.Object{v})
	if err != nil {
		return false, err
	}
	return objects.TruthOf(r)
}

// filterfalseIter yields the elements the predicate rejects; a nil predicate is
// the filterfalse(None, ...) form that keeps the falsy elements.
type filterfalseIter struct {
	pred objects.Object
	it   objects.Iterator
}

func (f *filterfalseIter) TypeName() string                   { return "filterfalse" }
func (f *filterfalseIter) Iterate() (objects.Iterator, error) { return f, nil }

func (f *filterfalseIter) Next() (objects.Object, bool, error) {
	for {
		v, ok, err := f.it.Next()
		if err != nil || !ok {
			return nil, false, err
		}
		keep := v
		if f.pred != nil {
			keep, err = objects.Call(f.pred, []objects.Object{v})
			if err != nil {
				return nil, false, err
			}
		}
		t, err := objects.TruthOf(keep)
		if err != nil {
			return nil, false, err
		}
		if !t {
			return v, true, nil
		}
	}
}

// starmapIter calls fn(*row) for each row the source yields.
type starmapIter struct {
	fn objects.Object
	it objects.Iterator
}

func (s *starmapIter) TypeName() string                   { return "starmap" }
func (s *starmapIter) Iterate() (objects.Iterator, error) { return s, nil }

func (s *starmapIter) Next() (objects.Object, bool, error) {
	row, ok, err := s.it.Next()
	if err != nil || !ok {
		return nil, false, err
	}
	args, err := materialize(row)
	if err != nil {
		return nil, false, err
	}
	r, err := objects.Call(s.fn, args)
	if err != nil {
		return nil, false, err
	}
	return r, true, nil
}

// accumulateIter yields the running totals. With no function it sums with +;
// with a function it folds left. An initial value is yielded first and seeds the
// fold.
type accumulateIter struct {
	it       objects.Iterator
	fn       objects.Object
	total    objects.Object
	hasTotal bool
	primed   bool
}

func (a *accumulateIter) TypeName() string                   { return "accumulate" }
func (a *accumulateIter) Iterate() (objects.Iterator, error) { return a, nil }

func (a *accumulateIter) Next() (objects.Object, bool, error) {
	if !a.primed {
		a.primed = true
		// An initial value is yielded as the first total; otherwise the first
		// element of the source seeds the running total and is yielded as is.
		if a.hasTotal {
			return a.total, true, nil
		}
		v, ok, err := a.it.Next()
		if err != nil || !ok {
			return nil, false, err
		}
		a.total = v
		a.hasTotal = true
		return v, true, nil
	}
	v, ok, err := a.it.Next()
	if err != nil || !ok {
		return nil, false, err
	}
	var next objects.Object
	if a.fn != nil {
		next, err = objects.Call(a.fn, []objects.Object{a.total, v})
	} else {
		next, err = objects.Add(a.total, v)
	}
	if err != nil {
		return nil, false, err
	}
	a.total = next
	return next, true, nil
}

// pairwiseIter yields overlapping (prev, cur) pairs. Fewer than two elements
// yields nothing.
type pairwiseIter struct {
	it      objects.Iterator
	prev    objects.Object
	hasPrev bool
}

func (p *pairwiseIter) TypeName() string                   { return "pairwise" }
func (p *pairwiseIter) Iterate() (objects.Iterator, error) { return p, nil }

func (p *pairwiseIter) Next() (objects.Object, bool, error) {
	if !p.hasPrev {
		v, ok, err := p.it.Next()
		if err != nil || !ok {
			return nil, false, err
		}
		p.prev = v
		p.hasPrev = true
	}
	v, ok, err := p.it.Next()
	if err != nil || !ok {
		return nil, false, err
	}
	pair := objects.NewTuple([]objects.Object{p.prev, v})
	p.prev = v
	return pair, true, nil
}

// zipLongestIter yields tuples across the inputs, substituting fillvalue for
// inputs that ran out and stopping once every input is exhausted.
type zipLongestIter struct {
	iters []objects.Iterator
	spent []bool
	fill  objects.Object
	live  int
}

func (z *zipLongestIter) TypeName() string                   { return "zip_longest" }
func (z *zipLongestIter) Iterate() (objects.Iterator, error) { return z, nil }

func (z *zipLongestIter) Next() (objects.Object, bool, error) {
	if len(z.iters) == 0 || z.live == 0 {
		return nil, false, nil
	}
	if z.spent == nil {
		z.spent = make([]bool, len(z.iters))
	}
	row := make([]objects.Object, len(z.iters))
	for i, it := range z.iters {
		if z.spent[i] {
			row[i] = z.fill
			continue
		}
		v, ok, err := it.Next()
		if err != nil {
			return nil, false, err
		}
		if !ok {
			z.spent[i] = true
			z.live--
			row[i] = z.fill
			continue
		}
		row[i] = v
	}
	if z.live == 0 {
		return nil, false, nil
	}
	return objects.NewTuple(row), true, nil
}

// itertoolsTee implements tee(iterable, n=2): n independent iterators reading a
// shared, lazily grown buffer of the source.
func itertoolsTee(a []objects.Object) (objects.Object, error) {
	if len(a) < 1 || len(a) > 2 {
		return nil, objects.Raise(objects.TypeError, "tee expected 1 to 2 arguments, got %d", len(a))
	}
	n := int64(2)
	if len(a) == 2 {
		var err error
		n, err = asIndex(a[1])
		if err != nil {
			return nil, err
		}
		if n < 0 {
			return nil, objects.Raise(objects.ValueError, "n must be >= 0")
		}
	}
	it, err := objects.Iter(a[0])
	if err != nil {
		return nil, err
	}
	buf := &teeBuffer{src: it}
	outs := make([]objects.Object, n)
	for i := range outs {
		outs[i] = &teeReader{buf: buf}
	}
	return objects.NewTuple(outs), nil
}

// teeBuffer holds the source and every element pulled so far, so each reader can
// re-read from its own cursor without advancing the others.
type teeBuffer struct {
	src   objects.Iterator
	items []objects.Object
	done  bool
	err   error
}

func (b *teeBuffer) at(i int) (objects.Object, bool, error) {
	for i >= len(b.items) {
		if b.done {
			return nil, false, b.err
		}
		v, ok, err := b.src.Next()
		if err != nil {
			b.done, b.err = true, err
			return nil, false, err
		}
		if !ok {
			b.done = true
			return nil, false, nil
		}
		b.items = append(b.items, v)
	}
	return b.items[i], true, nil
}

type teeReader struct {
	buf *teeBuffer
	i   int
}

func (t *teeReader) TypeName() string                   { return "_tee" }
func (t *teeReader) Iterate() (objects.Iterator, error) { return t, nil }

func (t *teeReader) Next() (objects.Object, bool, error) {
	v, ok, err := t.buf.at(t.i)
	if err != nil || !ok {
		return nil, false, err
	}
	t.i++
	return v, true, nil
}

// groupbyIter yields (key, group) pairs, each group an iterator over the run of
// consecutive elements sharing a key. A group is valid only until the next one
// is requested, matching CPython's shared-cursor design.
type groupbyIter struct {
	it        objects.Iterator
	keyfn     objects.Object
	currkey   objects.Object
	currval   objects.Object
	tgtkey    objects.Object
	started   bool
	tgtValid  bool
	exhausted bool
	id        int64
}

func (g *groupbyIter) TypeName() string                   { return "groupby" }
func (g *groupbyIter) Iterate() (objects.Iterator, error) { return g, nil }

func (g *groupbyIter) key(v objects.Object) (objects.Object, error) {
	if g.keyfn == nil {
		return v, nil
	}
	return objects.Call(g.keyfn, []objects.Object{v})
}

// sameKey reports whether the current key matches the target of the open group.
// Before the first fetch, or once the source is spent, there is no match.
func (g *groupbyIter) sameKey() (bool, error) {
	if !g.started || !g.tgtValid || g.exhausted {
		return false, nil
	}
	r, err := objects.Compare(objects.OpEq, g.currkey, g.tgtkey)
	if err != nil {
		return false, err
	}
	return objects.TruthOf(r)
}

func (g *groupbyIter) advance() (bool, error) {
	v, ok, err := g.it.Next()
	if err != nil {
		return false, err
	}
	if !ok {
		g.exhausted = true
		return false, nil
	}
	k, err := g.key(v)
	if err != nil {
		return false, err
	}
	g.currval, g.currkey, g.started = v, k, true
	return true, nil
}

func (g *groupbyIter) Next() (objects.Object, bool, error) {
	if g.exhausted {
		return nil, false, nil
	}
	g.id++
	for {
		same, err := g.sameKey()
		if err != nil {
			return nil, false, err
		}
		if !g.started {
			// prime the first element
		} else if !same {
			break
		}
		ok, err := g.advance()
		if err != nil {
			return nil, false, err
		}
		if !ok {
			return nil, false, nil
		}
	}
	g.tgtkey, g.tgtValid = g.currkey, true
	grp := &groupbyGrouper{g: g, id: g.id}
	return objects.NewTuple([]objects.Object{g.currkey, grp}), true, nil
}

// groupbyGrouper walks one group. It stops when the parent moves on to a new
// group or the shared cursor leaves the target key.
type groupbyGrouper struct {
	g  *groupbyIter
	id int64
}

func (gr *groupbyGrouper) TypeName() string                   { return "groupby" }
func (gr *groupbyGrouper) Iterate() (objects.Iterator, error) { return gr, nil }

func (gr *groupbyGrouper) Next() (objects.Object, bool, error) {
	if gr.id != gr.g.id {
		return nil, false, nil
	}
	same, err := gr.g.sameKey()
	if err != nil {
		return nil, false, err
	}
	if !same {
		return nil, false, nil
	}
	v := gr.g.currval
	if _, err := gr.g.advance(); err != nil {
		return nil, false, err
	}
	return v, true, nil
}

// batchedIter yields tuples of up to n consecutive elements. In strict mode a
// final short batch raises instead of being yielded.
type batchedIter struct {
	it     objects.Iterator
	n      int
	strict bool
	done   bool
}

func (b *batchedIter) TypeName() string                   { return "batched" }
func (b *batchedIter) Iterate() (objects.Iterator, error) { return b, nil }

func (b *batchedIter) Next() (objects.Object, bool, error) {
	if b.done {
		return nil, false, nil
	}
	batch := make([]objects.Object, 0, b.n)
	for len(batch) < b.n {
		v, ok, err := b.it.Next()
		if err != nil {
			return nil, false, err
		}
		if !ok {
			b.done = true
			break
		}
		batch = append(batch, v)
	}
	if len(batch) == 0 {
		return nil, false, nil
	}
	if b.done && b.strict && len(batch) != b.n {
		return nil, false, objects.Raise(objects.ValueError, "batched(): incomplete batch")
	}
	return objects.NewTuple(batch), true, nil
}

// itertoolsProduct implements product(*iterables, repeat=n) as a lazy odometer
// over the materialized pools.
func itertoolsProduct(a []objects.Object) (objects.Object, error) {
	repeat, err := asIndex(a[1])
	if err != nil {
		return nil, err
	}
	if repeat < 0 {
		return nil, objects.Raise(objects.ValueError, "repeat argument cannot be negative")
	}
	base := tupleElems(a[0])
	pools := make([][]objects.Object, 0, len(base)*int(repeat))
	for range int(repeat) {
		for _, src := range base {
			pool, err := materialize(src)
			if err != nil {
				return nil, err
			}
			pools = append(pools, pool)
		}
	}
	p := &productIter{pools: pools, idx: make([]int, len(pools))}
	for _, pool := range pools {
		if len(pool) == 0 {
			p.done = true // an empty pool makes the whole product empty
			break
		}
	}
	return p, nil
}

// productIter walks the cartesian product by incrementing an odometer of pool
// indices, rightmost first, the order itertools.product yields.
type productIter struct {
	pools [][]objects.Object
	idx   []int
	done  bool
}

func (p *productIter) TypeName() string                   { return "product" }
func (p *productIter) Iterate() (objects.Iterator, error) { return p, nil }

func (p *productIter) Next() (objects.Object, bool, error) {
	if p.done {
		return nil, false, nil
	}
	row := make([]objects.Object, len(p.pools))
	for i, pool := range p.pools {
		row[i] = pool[p.idx[i]]
	}
	// advance the odometer
	for i := len(p.idx) - 1; i >= 0; i-- {
		p.idx[i]++
		if p.idx[i] < len(p.pools[i]) {
			break
		}
		p.idx[i] = 0
		if i == 0 {
			p.done = true
		}
	}
	if len(p.pools) == 0 {
		p.done = true
	}
	return objects.NewTuple(row), true, nil
}

// itertoolsPermutations implements permutations(iterable, r=None).
func itertoolsPermutations(a []objects.Object) (objects.Object, error) {
	pool, err := materialize(a[0])
	if err != nil {
		return nil, err
	}
	n := len(pool)
	r := n
	if a[1] != objects.None {
		rr, err := asIndex(a[1])
		if err != nil {
			return nil, err
		}
		if rr < 0 {
			return nil, objects.Raise(objects.ValueError, "r must be non-negative")
		}
		r = int(rr)
	}
	p := &permutationsIter{pool: pool, r: r}
	if r <= n {
		p.indices = make([]int, n)
		for i := range p.indices {
			p.indices[i] = i
		}
		p.cycles = make([]int, r)
		for i := range p.cycles {
			p.cycles[i] = n - i
		}
		p.first = true
	} else {
		p.done = true
	}
	return p, nil
}

// permutationsIter follows CPython's index-and-cycle algorithm so the yield
// order matches itertools.permutations exactly.
type permutationsIter struct {
	pool    []objects.Object
	r       int
	indices []int
	cycles  []int
	first   bool
	done    bool
}

func (p *permutationsIter) TypeName() string                   { return "permutations" }
func (p *permutationsIter) Iterate() (objects.Iterator, error) { return p, nil }

func (p *permutationsIter) Next() (objects.Object, bool, error) {
	if p.done {
		return nil, false, nil
	}
	if p.first {
		p.first = false
		return p.emit(), true, nil
	}
	n := len(p.pool)
	for i := p.r - 1; i >= 0; i-- {
		p.cycles[i]--
		if p.cycles[i] == 0 {
			idx := p.indices[i]
			copy(p.indices[i:], p.indices[i+1:])
			p.indices[n-1] = idx
			p.cycles[i] = n - i
		} else {
			j := n - p.cycles[i]
			p.indices[i], p.indices[j] = p.indices[j], p.indices[i]
			return p.emit(), true, nil
		}
	}
	p.done = true
	return nil, false, nil
}

func (p *permutationsIter) emit() objects.Object {
	row := make([]objects.Object, p.r)
	for i := 0; i < p.r; i++ {
		row[i] = p.pool[p.indices[i]]
	}
	return objects.NewTuple(row)
}

// itertoolsCombinations implements combinations and, with replacement true,
// combinations_with_replacement.
func itertoolsCombinations(iterable, rObj objects.Object, withRepl bool) (objects.Object, error) {
	pool, err := materialize(iterable)
	if err != nil {
		return nil, err
	}
	r, err := asIndex(rObj)
	if err != nil {
		return nil, err
	}
	if r < 0 {
		return nil, objects.Raise(objects.ValueError, "r must be non-negative")
	}
	c := &combinationsIter{pool: pool, r: int(r), withRepl: withRepl}
	n := len(pool)
	if withRepl {
		if n == 0 && r > 0 {
			c.done = true
		}
	} else if int(r) > n {
		c.done = true
	}
	if !c.done {
		c.indices = make([]int, r)
		if !withRepl {
			for i := range c.indices {
				c.indices[i] = i
			}
		}
		c.first = true
	}
	return c, nil
}

// combinationsIter follows CPython's index algorithm for both the plain and
// with-replacement forms.
type combinationsIter struct {
	pool     []objects.Object
	r        int
	withRepl bool
	indices  []int
	first    bool
	done     bool
}

func (c *combinationsIter) TypeName() string                   { return "combinations" }
func (c *combinationsIter) Iterate() (objects.Iterator, error) { return c, nil }

func (c *combinationsIter) Next() (objects.Object, bool, error) {
	if c.done {
		return nil, false, nil
	}
	if c.first {
		c.first = false
		return c.emit(), true, nil
	}
	n := len(c.pool)
	i := c.r - 1
	for ; i >= 0; i-- {
		if c.withRepl {
			if c.indices[i] != n-1 {
				break
			}
		} else if c.indices[i] != i+n-c.r {
			break
		}
	}
	if i < 0 {
		c.done = true
		return nil, false, nil
	}
	if c.withRepl {
		v := c.indices[i] + 1
		for j := i; j < c.r; j++ {
			c.indices[j] = v
		}
	} else {
		c.indices[i]++
		for j := i + 1; j < c.r; j++ {
			c.indices[j] = c.indices[j-1] + 1
		}
	}
	return c.emit(), true, nil
}

func (c *combinationsIter) emit() objects.Object {
	row := make([]objects.Object, c.r)
	for i := 0; i < c.r; i++ {
		row[i] = c.pool[c.indices[i]]
	}
	return objects.NewTuple(row)
}
