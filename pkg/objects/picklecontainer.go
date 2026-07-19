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
// Sets and frozensets need CPython's set-iteration order to stay byte-identical
// and arrive in a later slice; dict subclasses and namedtuples pickle through
// the object-reduction protocol, also later.

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

	pickleBatchSize = 1000 // CPython's _BATCHSIZE for APPENDS/SETITEMS chunks
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
