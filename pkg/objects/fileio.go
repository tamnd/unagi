package objects

import "fmt"

// StringIO and BytesIO are the in-memory streams the io module exposes. They
// are C types in CPython, so the runtime provides them in Go behind the io
// import. Both hold a growable buffer and a read/write cursor, and both are
// their own context managers: `with io.StringIO() as f` enters to the stream
// and closes it on the way out.
//
// A StringIO counts its cursor in code points, matching CPython, where tell()
// and read(n) are measured in characters, not bytes. A BytesIO counts bytes.

type stringIOObject struct {
	buf    []rune
	pos    int
	closed bool
}

type bytesIOObject struct {
	buf    []byte
	pos    int
	closed bool
}

// NewStringIO builds an io.StringIO over the initial value.
func NewStringIO(initial string) Object {
	return &stringIOObject{buf: []rune(initial)}
}

// NewBytesIO builds an io.BytesIO over the initial bytes.
func NewBytesIO(initial []byte) Object {
	b := make([]byte, len(initial))
	copy(b, initial)
	return &bytesIOObject{buf: b}
}

func (s *stringIOObject) TypeName() string { return "_io.StringIO" }
func (b *bytesIOObject) TypeName() string  { return "_io.BytesIO" }

// checkOpen guards every operation that touches a closed stream with the
// message CPython raises.
func fileClosed() error {
	return Raise(ValueError, "I/O operation on closed file")
}

// seekPos resolves a seek target against the current length for whence 0, 1, 2,
// the three modes StringIO and BytesIO both accept.
func seekPos(cur, length int, args []Object) (int, error) {
	if len(args) < 1 || len(args) > 2 {
		return 0, Raise(TypeError, "seek() takes at least 1 argument (%d given)", len(args))
	}
	off, ok := AsInt(args[0])
	if !ok {
		return 0, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
	}
	whence := int64(0)
	if len(args) == 2 {
		w, ok := AsInt(args[1])
		if !ok {
			return 0, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[1].TypeName())
		}
		whence = w
	}
	var base int64
	switch whence {
	case 0:
		base = 0
	case 1:
		base = int64(cur)
	case 2:
		base = int64(length)
	default:
		return 0, Raise(ValueError, "invalid whence (%d, should be 0, 1 or 2)", whence)
	}
	target := base + off
	if target < 0 {
		return 0, Raise(ValueError, "negative seek position %d", target)
	}
	return int(target), nil
}

func stringIOMethod(s *stringIOObject, name string, args []Object) (Object, error) {
	switch name {
	case "write":
		if s.closed {
			return nil, fileClosed()
		}
		if len(args) != 1 {
			return nil, Raise(TypeError, "write() takes exactly one argument (%d given)", len(args))
		}
		str, ok := args[0].(*strObject)
		if !ok {
			return nil, Raise(TypeError, "string argument expected, got '%s'", args[0].TypeName())
		}
		w := []rune(str.v)
		s.growTo(s.pos)
		// Overwrite from the cursor, extending the buffer when the write runs
		// past the current end.
		for i, r := range w {
			if s.pos+i < len(s.buf) {
				s.buf[s.pos+i] = r
			} else {
				s.buf = append(s.buf, r)
			}
		}
		s.pos += len(w)
		return NewInt(int64(len(w))), nil
	case "read":
		if s.closed {
			return nil, fileClosed()
		}
		n, err := readCount(args, len(s.buf)-s.pos)
		if err != nil {
			return nil, err
		}
		out := string(s.buf[s.pos : s.pos+n])
		s.pos += n
		return NewStr(out), nil
	case "readline":
		if s.closed {
			return nil, fileClosed()
		}
		out := s.readlineRunes(args)
		return NewStr(string(out)), nil
	case "getvalue":
		if s.closed {
			return nil, fileClosed()
		}
		return NewStr(string(s.buf)), nil
	case "tell":
		if s.closed {
			return nil, fileClosed()
		}
		return NewInt(int64(s.pos)), nil
	case "seek":
		if s.closed {
			return nil, fileClosed()
		}
		p, err := seekPos(s.pos, len(s.buf), args)
		if err != nil {
			return nil, err
		}
		s.pos = p
		return NewInt(int64(p)), nil
	case "truncate":
		if s.closed {
			return nil, fileClosed()
		}
		size := s.pos
		if len(args) == 1 && args[0] != None {
			n, ok := AsInt(args[0])
			if !ok {
				return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
			}
			size = int(n)
		}
		if size < len(s.buf) {
			s.buf = s.buf[:size]
		}
		return NewInt(int64(size)), nil
	case "writelines":
		return writeLines(s, args)
	case "close":
		s.closed = true
		return None, nil
	case "flush":
		if s.closed {
			return nil, fileClosed()
		}
		return None, nil
	case "readable", "writable", "seekable":
		if s.closed {
			return nil, fileClosed()
		}
		return NewBool(true), nil
	case "__enter__":
		if s.closed {
			return nil, fileClosed()
		}
		return s, nil
	case "__exit__":
		s.closed = true
		return None, nil
	}
	return nil, noAttr(s, name)
}

// growTo pads the buffer with spaces up to n so a seek past the end followed by
// a write leaves the gap filled the way CPython does with NUL-free padding.
func (s *stringIOObject) growTo(n int) {
	for len(s.buf) < n {
		s.buf = append(s.buf, ' ')
	}
}

func (s *stringIOObject) readlineRunes(args []Object) []rune {
	limit := -1
	if len(args) == 1 && args[0] != None {
		if n, ok := AsInt(args[0]); ok {
			limit = int(n)
		}
	}
	start := s.pos
	for s.pos < len(s.buf) {
		if limit >= 0 && s.pos-start >= limit {
			break
		}
		r := s.buf[s.pos]
		s.pos++
		if r == '\n' {
			break
		}
	}
	return s.buf[start:s.pos]
}

func bytesIOMethod(b *bytesIOObject, name string, args []Object) (Object, error) {
	switch name {
	case "write":
		if b.closed {
			return nil, fileClosed()
		}
		if len(args) != 1 {
			return nil, Raise(TypeError, "write() takes exactly one argument (%d given)", len(args))
		}
		data, ok := asBytesLike(args[0])
		if !ok {
			return nil, Raise(TypeError, "a bytes-like object is required, not '%s'", args[0].TypeName())
		}
		for len(b.buf) < b.pos {
			b.buf = append(b.buf, 0)
		}
		for i, c := range data {
			if b.pos+i < len(b.buf) {
				b.buf[b.pos+i] = c
			} else {
				b.buf = append(b.buf, c)
			}
		}
		b.pos += len(data)
		return NewInt(int64(len(data))), nil
	case "read":
		if b.closed {
			return nil, fileClosed()
		}
		n, err := readCount(args, len(b.buf)-b.pos)
		if err != nil {
			return nil, err
		}
		out := make([]byte, n)
		copy(out, b.buf[b.pos:b.pos+n])
		b.pos += n
		return NewBytes(out), nil
	case "getvalue":
		if b.closed {
			return nil, fileClosed()
		}
		out := make([]byte, len(b.buf))
		copy(out, b.buf)
		return NewBytes(out), nil
	case "tell":
		if b.closed {
			return nil, fileClosed()
		}
		return NewInt(int64(b.pos)), nil
	case "seek":
		if b.closed {
			return nil, fileClosed()
		}
		p, err := seekPos(b.pos, len(b.buf), args)
		if err != nil {
			return nil, err
		}
		b.pos = p
		return NewInt(int64(p)), nil
	case "truncate":
		if b.closed {
			return nil, fileClosed()
		}
		size := b.pos
		if len(args) == 1 && args[0] != None {
			n, ok := AsInt(args[0])
			if !ok {
				return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
			}
			size = int(n)
		}
		if size < len(b.buf) {
			b.buf = b.buf[:size]
		}
		return NewInt(int64(size)), nil
	case "close":
		b.closed = true
		return None, nil
	case "flush":
		if b.closed {
			return nil, fileClosed()
		}
		return None, nil
	case "readable", "writable", "seekable":
		if b.closed {
			return nil, fileClosed()
		}
		return NewBool(true), nil
	case "__enter__":
		if b.closed {
			return nil, fileClosed()
		}
		return b, nil
	case "__exit__":
		b.closed = true
		return None, nil
	}
	return nil, noAttr(b, name)
}

// readCount resolves a read(size) argument: a missing or negative size reads
// everything remaining, which is avail here.
func readCount(args []Object, avail int) (int, error) {
	if len(args) == 0 {
		return avail, nil
	}
	if len(args) > 1 {
		return 0, Raise(TypeError, "read() takes at most 1 argument (%d given)", len(args))
	}
	if args[0] == None {
		return avail, nil
	}
	n, ok := AsInt(args[0])
	if !ok {
		return 0, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
	}
	if n < 0 || int(n) > avail {
		return avail, nil
	}
	return int(n), nil
}

// writeLines writes each item of an iterable in turn, the writelines the
// text and binary streams share.
func writeLines(s Object, args []Object) (Object, error) {
	if len(args) != 1 {
		return nil, Raise(TypeError, "writelines() takes exactly one argument (%d given)", len(args))
	}
	it, err := Iter(args[0])
	if err != nil {
		return nil, err
	}
	for {
		item, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		if _, err := CallMethod(s, "write", []Object{item}); err != nil {
			return nil, err
		}
	}
	return None, nil
}

func stringIORepr(s *stringIOObject) string { return fmt.Sprintf("<_io.StringIO object at %p>", s) }
func bytesIORepr(b *bytesIOObject) string   { return fmt.Sprintf("<_io.BytesIO object at %p>", b) }

// supportsNativeCM reports whether a non-instance object drives the context
// manager protocol through CallMethod, the way the in-memory streams do.
func supportsNativeCM(o Object) bool {
	switch o.(type) {
	case *stringIOObject, *bytesIOObject, *lockObject, *rlockObject, *condObject, *semaphoreObject, *executorObject, *asyncioRunner:
		return true
	}
	return false
}

// Iterate lets `for line in f` walk a StringIO, consuming it line by line and
// leaving the cursor where iteration stopped, the way a file does.
func (s *stringIOObject) Iterate() (Iterator, error) {
	if s.closed {
		return nil, fileClosed()
	}
	return &stringIOIter{s: s}, nil
}

type stringIOIter struct{ s *stringIOObject }

func (it *stringIOIter) Next() (Object, bool, error) {
	if it.s.closed {
		return nil, false, fileClosed()
	}
	if it.s.pos >= len(it.s.buf) {
		return nil, false, nil
	}
	line := it.s.readlineRunes(nil)
	return NewStr(string(line)), true, nil
}

// Iterate walks a BytesIO by line, splitting on the newline byte.
func (b *bytesIOObject) Iterate() (Iterator, error) {
	if b.closed {
		return nil, fileClosed()
	}
	return &bytesIOIter{b: b}, nil
}

type bytesIOIter struct{ b *bytesIOObject }

func (it *bytesIOIter) Next() (Object, bool, error) {
	if it.b.closed {
		return nil, false, fileClosed()
	}
	buf := it.b
	if buf.pos >= len(buf.buf) {
		return nil, false, nil
	}
	start := buf.pos
	for buf.pos < len(buf.buf) {
		c := buf.buf[buf.pos]
		buf.pos++
		if c == '\n' {
			break
		}
	}
	out := make([]byte, buf.pos-start)
	copy(out, buf.buf[start:buf.pos])
	return NewBytes(out), true, nil
}
