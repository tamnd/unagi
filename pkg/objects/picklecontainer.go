package objects

// Container pickling extends the scalar pickler to the built-in ordered
// containers — tuple, list, and dict — reproducing CPython's opcode selection
// and memo discipline byte for byte. The rules that matter for the exact bytes:
//
//   - A mutable container (list, dict) is memoized *before* its contents, so a
//     shared or self-referential child fetches it back with a memo GET instead
//     of re-serializing, and a cycle terminates.
//   - An immutable tuple is memoized *after* its contents, since it does not
//     exist until they are built; the empty tuple is a singleton and never
//     memoized.
//   - Contents go out in batches of pickleBatchSize: a batch of more than one
//     item is framed by MARK ... APPENDS/SETITEMS, a lone item takes the
//     single-item APPEND/SETITEM, matching CPython's _batch_appends /
//     _batch_setitems exactly.
//
// A set and a frozenset stay byte-identical because the harness pins
// PYTHONHASHSEED=0 and PyHash reproduces those hashes, so CPython's set
// iteration order — the hash-table slot order — is deterministic and
// reproducible; cpythonSetOrder rebuilds it. Only protocol 4+ has the
// EMPTY_SET/FROZENSET opcodes; protocols 2 and 3 pickle a set through the
// object-reduction protocol (a set() global applied to a list), which lands
// with that machinery in a later slice. Dict subclasses and namedtuples
// reduce as well, also later.

// Container opcodes, spelled as CPython's pickle module names them.
const (
	opMark       = '('  // 0x28 push a mark to bound a variable-length build
	opEmptyTuple = ')'  // 0x29 the empty tuple singleton
	opTuple      = 't'  // 0x74 build a tuple from everything back to the mark
	opTuple1     = 0x85 // build a 1-tuple from the top item
	opTuple2     = 0x86 // build a 2-tuple from the top two items
	opTuple3     = 0x87 // build a 3-tuple from the top three items
	opEmptyList  = ']'  // 0x5d a fresh empty list
	opAppend     = 'a'  // 0x61 append the top item to the list below it
	opAppends    = 'e'  // 0x65 append everything back to the mark
	opEmptyDict  = '}'  // 0x7d a fresh empty dict
	opSetItem    = 's'  // 0x73 set one key/value on the dict below them
	opSetItems   = 'u'  // 0x75 set every key/value pair back to the mark
	opEmptySet   = 0x8f // a fresh empty set (protocol 4+)
	opAddItems   = 0x90 // add everything back to the mark to the set below it
	opFrozenset  = 0x91 // build a frozenset from everything back to the mark

	pickleBatchSize = 1000 // CPython's _BATCHSIZE for APPENDS/SETITEMS chunks
)

// CPython setobject.c layout constants, replayed by cpythonSetOrder.
const (
	pickleSetMinSize      = 8 // PySet_MINSIZE, the initial table size
	pickleSetLinearProbes = 9 // LINEAR_PROBES, the contiguous scan before perturbing
	pickleSetPerturbShift = 5 // PERTURB_SHIFT, folds high hash bits into the probe
)

// saveTuple writes a tuple. The empty tuple is the singleton EMPTY_TUPLE; one to
// three elements use the fixed TUPLE1/2/3 opcodes; longer tuples are framed by
// MARK ... TUPLE. The tuple is memoized after its elements, and a tuple already
// in the memo (a shared reference) is fetched rather than rewritten.
func (p *pickler) saveTuple(v *tupleObject, o Object) error {
	n := len(v.elts)
	if n == 0 {
		p.framer.write(opEmptyTuple)
		return nil
	}
	if p.memoGet(o) {
		return nil
	}
	if n <= 3 {
		for _, e := range v.elts {
			if err := p.save(e); err != nil {
				return err
			}
		}
		switch n {
		case 1:
			p.framer.write(opTuple1)
		case 2:
			p.framer.write(opTuple2)
		case 3:
			p.framer.write(opTuple3)
		}
		p.memoize(o)
		return nil
	}
	p.framer.write(opMark)
	for _, e := range v.elts {
		if err := p.save(e); err != nil {
			return err
		}
	}
	p.framer.write(opTuple)
	p.memoize(o)
	return nil
}

// saveList writes a list: EMPTY_LIST, memoize (before contents so a cycle
// terminates), then the elements in APPEND/APPENDS batches.
func (p *pickler) saveList(v *listObject, o Object) error {
	if p.memoGet(o) {
		return nil
	}
	p.framer.write(opEmptyList)
	p.memoize(o)
	return p.batchAppends(v.elts)
}

// batchAppends writes list contents the way CPython's _batch_appends does: up to
// pickleBatchSize items per MARK ... APPENDS group, but a lone item in a group
// takes the single-item APPEND. An empty list writes nothing.
func (p *pickler) batchAppends(items []Object) error {
	i := 0
	for {
		end := i + pickleBatchSize
		if end > len(items) {
			end = len(items)
		}
		n := end - i
		if n > 1 {
			p.framer.write(opMark)
			for _, e := range items[i:end] {
				if err := p.save(e); err != nil {
					return err
				}
			}
			p.framer.write(opAppends)
		} else if n == 1 {
			if err := p.save(items[i]); err != nil {
				return err
			}
			p.framer.write(opAppend)
		}
		i = end
		if n < pickleBatchSize {
			return nil
		}
	}
}

// saveDict writes a dict: EMPTY_DICT, memoize (before contents), then the
// key/value pairs in SETITEM/SETITEMS batches, in insertion order.
func (p *pickler) saveDict(v *dictObject, o Object) error {
	if p.memoGet(o) {
		return nil
	}
	p.framer.write(opEmptyDict)
	p.memoize(o)
	return p.batchSetItems(v.entries)
}

// batchSetItems writes dict contents the way CPython's _batch_setitems does: up
// to pickleBatchSize pairs per MARK ... SETITEMS group, a lone pair taking the
// single-item SETITEM. An empty dict writes nothing.
func (p *pickler) batchSetItems(entries []dictEntry) error {
	i := 0
	for {
		end := i + pickleBatchSize
		if end > len(entries) {
			end = len(entries)
		}
		n := end - i
		if n > 1 {
			p.framer.write(opMark)
			for _, e := range entries[i:end] {
				if err := p.save(e.key); err != nil {
					return err
				}
				if err := p.save(e.val); err != nil {
					return err
				}
			}
			p.framer.write(opSetItems)
		} else if n == 1 {
			if err := p.save(entries[i].key); err != nil {
				return err
			}
			if err := p.save(entries[i].val); err != nil {
				return err
			}
			p.framer.write(opSetItem)
		}
		i = end
		if n < pickleBatchSize {
			return nil
		}
	}
}

// saveSet writes a set: EMPTY_SET, memoize (before contents, so a set shared
// through a container fetches back with a memo GET), then its elements in
// MARK ... ADDITEMS batches, in CPython's set-iteration order. Protocols below
// 4 have no EMPTY_SET opcode and pickle a set through the reduction protocol
// instead, which this slice does not emit yet.
func (p *pickler) saveSet(v *setObject, o Object) error {
	if p.proto < 4 {
		return Raise("NotImplementedError",
			"pickling a set at protocol %d needs the object-reduction protocol, which is not supported yet; use protocol 4 or higher", p.proto)
	}
	if p.memoGet(o) {
		return nil
	}
	order, err := cpythonSetOrder(&v.setCore)
	if err != nil {
		return err
	}
	p.framer.write(opEmptySet)
	p.memoize(o)
	return p.batchAddItems(v.elts, order)
}

// saveFrozenset writes a frozenset: MARK, its elements in set-iteration order,
// then FROZENSET, memoized after (it is immutable and does not exist until its
// members are built). A frozenset already in the memo — the same object shared
// through a container — fetches back instead. A frozenset cannot take part in a
// reference cycle (its members are all immutable and built before it), so no
// mid-build recursion check is needed. Protocols below 4 reduce, a later slice.
func (p *pickler) saveFrozenset(v *frozensetObject, o Object) error {
	if p.proto < 4 {
		return Raise("NotImplementedError",
			"pickling a frozenset at protocol %d needs the object-reduction protocol, which is not supported yet; use protocol 4 or higher", p.proto)
	}
	if p.memoGet(o) {
		return nil
	}
	order, err := cpythonSetOrder(&v.setCore)
	if err != nil {
		return err
	}
	p.framer.write(opMark)
	for _, idx := range order {
		if err := p.save(v.elts[idx]); err != nil {
			return err
		}
	}
	p.framer.write(opFrozenset)
	p.memoize(o)
	return nil
}

// batchAddItems writes set contents the way CPython's save_set does: up to
// pickleBatchSize elements per MARK ... ADDITEMS group. Unlike a list, a set
// always frames a non-empty batch with MARK ... ADDITEMS — there is no
// single-item shortcut — and an empty set writes nothing. order carries the
// indices of items in CPython's iteration order.
func (p *pickler) batchAddItems(items []Object, order []int) error {
	i := 0
	for {
		end := i + pickleBatchSize
		if end > len(order) {
			end = len(order)
		}
		n := end - i
		if n > 0 {
			p.framer.write(opMark)
			for _, idx := range order[i:end] {
				if err := p.save(items[idx]); err != nil {
					return err
				}
			}
			p.framer.write(opAddItems)
		}
		i = end
		if n < pickleBatchSize {
			return nil
		}
	}
}

// cpythonSetOrder returns the indices of c.elts in the order CPython 3.14
// iterates the equivalent set: by hash-table slot. It replays setobject.c's
// insert-and-grow exactly — the same initial size, linear-probe window, perturb
// recurrence, load-factor resize, and re-insertion in old-slot order — over the
// elements in insertion order, using the hashes PyHash pins to PYTHONHASHSEED=0.
// The resulting slot walk is byte-for-byte what CPython's set and frozenset
// picklers and iterators emit.
func cpythonSetOrder(c *setCore) ([]int, error) {
	n := len(c.elts)
	hashes := make([]uint64, n)
	for i, e := range c.elts {
		h, err := PyHash(e)
		if err != nil {
			return nil, err
		}
		hashes[i] = uint64(h)
	}
	table := newPickleSetTable(pickleSetMinSize)
	mask := uint64(pickleSetMinSize - 1)
	fill := uint64(0)
	for idx := 0; idx < n; idx++ {
		pickleSetInsert(table, mask, idx, hashes[idx])
		fill++
		// CPython resizes once the table is three-fifths full, growing to four
		// times the live count (doubling from PySet_MINSIZE until it fits), and
		// re-inserts the survivors in old-slot order.
		if fill*5 >= mask*3 {
			minused := fill * 4
			if fill > 50000 {
				minused = fill * 2
			}
			newSize := uint64(pickleSetMinSize)
			for newSize <= minused {
				newSize <<= 1
			}
			grown := newPickleSetTable(newSize)
			nmask := newSize - 1
			for _, slot := range table {
				if slot >= 0 {
					pickleSetInsert(grown, nmask, slot, hashes[slot])
				}
			}
			table = grown
			mask = nmask
		}
	}
	order := make([]int, 0, n)
	for _, slot := range table {
		if slot >= 0 {
			order = append(order, slot)
		}
	}
	return order, nil
}

// newPickleSetTable returns a set table of the given size with every slot empty,
// an empty slot being -1.
func newPickleSetTable(size uint64) []int {
	table := make([]int, size)
	for i := range table {
		table[i] = -1
	}
	return table
}

// pickleSetInsert places element idx (with hash h) into a set table by CPython's
// probe sequence: try the home slot, then a contiguous run of LINEAR_PROBES
// slots while they stay within the table, then jump by the perturb recurrence
// and repeat. Every element is distinct, so the search only ever looks for the
// first empty slot.
func pickleSetInsert(table []int, mask uint64, idx int, h uint64) {
	perturb := h
	i := h & mask
	for {
		if table[i] < 0 {
			table[i] = idx
			return
		}
		if i+pickleSetLinearProbes <= mask {
			for j := uint64(1); j <= pickleSetLinearProbes; j++ {
				if table[i+j] < 0 {
					table[i+j] = idx
					return
				}
			}
		}
		perturb >>= pickleSetPerturbShift
		i = (i*5 + 1 + perturb) & mask
	}
}
