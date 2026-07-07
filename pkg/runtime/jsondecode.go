package runtime

import (
	"fmt"
	"math"
	"math/big"
	"strconv"

	"github.com/tamnd/unagi/pkg/objects"
)

// jsonDecodeErrorClass is the JSONDecodeError class object, a ValueError
// subclass built once when the json module executes. The decoder raises
// instances of it directly so both `except json.JSONDecodeError` and
// `except ValueError` catch a parse failure.
var jsonDecodeErrorClass objects.Object

// jsonScanner carries the document as a rune slice so positions count in code
// points, the unit CPython reports in the JSONDecodeError pos, lineno, and colno.
type jsonScanner struct {
	s []rune
}

// jsonLoads parses a full JSON document into Python objects. Trailing content
// after the top-level value is the "Extra data" error, matching json.loads.
func jsonLoads(doc string) (objects.Object, error) {
	sc := &jsonScanner{s: []rune(doc)}
	v, i, err := sc.value(sc.skipWS(0))
	if err != nil {
		return nil, err
	}
	i = sc.skipWS(i)
	if i != len(sc.s) {
		return nil, sc.err("Extra data", i)
	}
	return v, nil
}

func (sc *jsonScanner) skipWS(i int) int {
	for i < len(sc.s) {
		switch sc.s[i] {
		case ' ', '\t', '\n', '\r':
			i++
		default:
			return i
		}
	}
	return i
}

// value decodes one JSON value starting at i, which must already be past any
// leading whitespace. It returns the value and the index just after it.
func (sc *jsonScanner) value(i int) (objects.Object, int, error) {
	if i >= len(sc.s) {
		return nil, 0, sc.err("Expecting value", i)
	}
	switch c := sc.s[i]; c {
	case '"':
		s, j, err := sc.scanString(i)
		if err != nil {
			return nil, 0, err
		}
		return objects.NewStr(s), j, nil
	case '{':
		return sc.object(i)
	case '[':
		return sc.array(i)
	case 't':
		if sc.lit(i, "true") {
			return objects.True, i + 4, nil
		}
	case 'f':
		if sc.lit(i, "false") {
			return objects.False, i + 5, nil
		}
	case 'n':
		if sc.lit(i, "null") {
			return objects.None, i + 4, nil
		}
	case 'N':
		if sc.lit(i, "NaN") {
			return objects.NewFloat(math.NaN()), i + 3, nil
		}
	case 'I':
		if sc.lit(i, "Infinity") {
			return objects.NewFloat(math.Inf(1)), i + 8, nil
		}
	case '-':
		if sc.lit(i, "-Infinity") {
			return objects.NewFloat(math.Inf(-1)), i + 9, nil
		}
		return sc.number(i)
	}
	if c := sc.s[i]; c >= '0' && c <= '9' {
		return sc.number(i)
	}
	return nil, 0, sc.err("Expecting value", i)
}

// lit reports whether the literal word sits at i.
func (sc *jsonScanner) lit(i int, word string) bool {
	w := []rune(word)
	if i+len(w) > len(sc.s) {
		return false
	}
	for k, r := range w {
		if sc.s[i+k] != r {
			return false
		}
	}
	return true
}

// number matches the JSON number grammar and returns an int when there is no
// fraction or exponent, a float otherwise. A leading sign with no digit is not
// a number, so the caller reports "Expecting value".
func (sc *jsonScanner) number(i int) (objects.Object, int, error) {
	n := len(sc.s)
	start := i
	if i < n && sc.s[i] == '-' {
		i++
	}
	if i < n && sc.s[i] == '0' {
		i++
	} else if i < n && sc.s[i] >= '1' && sc.s[i] <= '9' {
		i++
		for i < n && sc.s[i] >= '0' && sc.s[i] <= '9' {
			i++
		}
	} else {
		return nil, 0, sc.err("Expecting value", start)
	}
	isFloat := false
	if i+1 < n && sc.s[i] == '.' && sc.s[i+1] >= '0' && sc.s[i+1] <= '9' {
		isFloat = true
		i += 2
		for i < n && sc.s[i] >= '0' && sc.s[i] <= '9' {
			i++
		}
	}
	if i < n && (sc.s[i] == 'e' || sc.s[i] == 'E') {
		j := i + 1
		if j < n && (sc.s[j] == '+' || sc.s[j] == '-') {
			j++
		}
		if j < n && sc.s[j] >= '0' && sc.s[j] <= '9' {
			for j < n && sc.s[j] >= '0' && sc.s[j] <= '9' {
				j++
			}
			isFloat = true
			i = j
		}
	}
	text := string(sc.s[start:i])
	if isFloat {
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			// Only overflow reaches here; CPython yields the infinity rather
			// than raising, so ParseFloat's inf result is what we want.
			f = math.Inf(1)
			if len(text) > 0 && text[0] == '-' {
				f = math.Inf(-1)
			}
		}
		return objects.NewFloat(f), i, nil
	}
	b, ok := new(big.Int).SetString(text, 10)
	if !ok {
		return nil, 0, sc.err("Expecting value", start)
	}
	return objects.NewIntFromBig(b), i, nil
}

// scanString decodes a JSON string whose opening quote is at i. It returns the
// decoded text and the index just past the closing quote. The error positions
// follow the C scanner: an unterminated string points at the opening quote, a
// control character or bad escape at the offending character.
func (sc *jsonScanner) scanString(i int) (string, int, error) {
	begin := i
	i++
	var out []rune
	n := len(sc.s)
	for {
		if i >= n {
			return "", 0, sc.err("Unterminated string starting at", begin)
		}
		c := sc.s[i]
		switch {
		case c == '"':
			return string(out), i + 1, nil
		case c == '\\':
			bs := i
			i++
			if i >= n {
				return "", 0, sc.err("Unterminated string starting at", begin)
			}
			e := sc.s[i]
			if e == 'u' {
				r, ok := sc.hex4(i + 1)
				if !ok {
					return "", 0, sc.err("Invalid \\uXXXX escape", i)
				}
				i += 5
				if r >= 0xD800 && r <= 0xDBFF && i+1 < n && sc.s[i] == '\\' && sc.s[i+1] == 'u' {
					if lo, ok := sc.hex4(i + 2); ok && lo >= 0xDC00 && lo <= 0xDFFF {
						r = 0x10000 + (r-0xD800)<<10 + (lo - 0xDC00)
						i += 6
					}
				}
				out = append(out, rune(r))
				continue
			}
			d, ok := jsonUnescape[e]
			if !ok {
				return "", 0, sc.err("Invalid \\escape", bs)
			}
			out = append(out, d)
			i++
		case c < 0x20:
			return "", 0, sc.err("Invalid control character at", i)
		default:
			out = append(out, c)
			i++
		}
	}
}

// jsonUnescape is the two-character escape table the JSON grammar allows.
var jsonUnescape = map[rune]rune{
	'"': '"', '\\': '\\', '/': '/',
	'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t',
}

// hex4 reads four hex digits at i and returns their value. ok is false when
// fewer than four digits are present or any is not hex.
func (sc *jsonScanner) hex4(i int) (int, bool) {
	if i+4 > len(sc.s) {
		return 0, false
	}
	v := 0
	for k := range 4 {
		d := sc.s[i+k]
		v <<= 4
		switch {
		case d >= '0' && d <= '9':
			v |= int(d - '0')
		case d >= 'a' && d <= 'f':
			v |= int(d-'a') + 10
		case d >= 'A' && d <= 'F':
			v |= int(d-'A') + 10
		default:
			return 0, false
		}
	}
	return v, true
}

// array decodes a JSON array whose opening bracket is at i.
func (sc *jsonScanner) array(i int) (objects.Object, int, error) {
	i = sc.skipWS(i + 1)
	if i < len(sc.s) && sc.s[i] == ']' {
		return objects.NewList(nil), i + 1, nil
	}
	var elts []objects.Object
	for {
		v, j, err := sc.value(sc.skipWS(i))
		if err != nil {
			return nil, 0, err
		}
		elts = append(elts, v)
		i = sc.skipWS(j)
		if i >= len(sc.s) {
			return nil, 0, sc.err("Expecting ',' delimiter", i)
		}
		switch sc.s[i] {
		case ']':
			return objects.NewList(elts), i + 1, nil
		case ',':
			comma := i
			i = sc.skipWS(i + 1)
			if i < len(sc.s) && sc.s[i] == ']' {
				return nil, 0, sc.err("Illegal trailing comma before end of array", comma)
			}
		default:
			return nil, 0, sc.err("Expecting ',' delimiter", i)
		}
	}
}

// object decodes a JSON object whose opening brace is at i. A duplicate key
// keeps the last value, matching CPython.
func (sc *jsonScanner) object(i int) (objects.Object, int, error) {
	i = sc.skipWS(i + 1)
	if i < len(sc.s) && sc.s[i] == '}' {
		d, err := objects.NewDict(nil, nil)
		return d, i + 1, err
	}
	var keys, vals []objects.Object
	for {
		if i >= len(sc.s) || sc.s[i] != '"' {
			return nil, 0, sc.err("Expecting property name enclosed in double quotes", i)
		}
		key, j, err := sc.scanString(i)
		if err != nil {
			return nil, 0, err
		}
		i = sc.skipWS(j)
		if i >= len(sc.s) || sc.s[i] != ':' {
			return nil, 0, sc.err("Expecting ':' delimiter", i)
		}
		v, j, err := sc.value(sc.skipWS(i + 1))
		if err != nil {
			return nil, 0, err
		}
		keys = append(keys, objects.NewStr(key))
		vals = append(vals, v)
		i = sc.skipWS(j)
		if i >= len(sc.s) {
			return nil, 0, sc.err("Expecting ',' delimiter", i)
		}
		switch sc.s[i] {
		case '}':
			d, err := objects.NewDict(keys, vals)
			return d, i + 1, err
		case ',':
			comma := i
			i = sc.skipWS(i + 1)
			if i < len(sc.s) && sc.s[i] == '}' {
				return nil, 0, sc.err("Illegal trailing comma before end of object", comma)
			}
		default:
			return nil, 0, sc.err("Expecting ',' delimiter", i)
		}
	}
}

// err builds a JSONDecodeError for msg at code-point position pos, computing
// lineno and colno the way JSONDecodeError.__init__ does and formatting the
// same "msg: line L column C (char P)" text into args so str(e) matches.
func (sc *jsonScanner) err(msg string, pos int) error {
	lineno := 1
	lastNL := -1
	for k := 0; k < pos && k < len(sc.s); k++ {
		if sc.s[k] == '\n' {
			lineno++
			lastNL = k
		}
	}
	colno := pos - lastNL
	errmsg := fmt.Sprintf("%s: line %d column %d (char %d)", msg, lineno, colno, pos)
	excObj, cerr := objects.Call(jsonDecodeErrorClass, []objects.Object{objects.NewStr(errmsg)})
	if cerr != nil {
		return cerr
	}
	doc := string(sc.s)
	for _, kv := range []struct {
		name string
		val  objects.Object
	}{
		{"msg", objects.NewStr(msg)},
		{"doc", objects.NewStr(doc)},
		{"pos", objects.NewInt(int64(pos))},
		{"lineno", objects.NewInt(int64(lineno))},
		{"colno", objects.NewInt(int64(colno))},
	} {
		if serr := objects.StoreAttr(excObj, kv.name, kv.val); serr != nil {
			return serr
		}
	}
	if e, ok := excObj.(error); ok {
		return e
	}
	return objects.Raise(objects.ValueError, "%s", errmsg)
}
