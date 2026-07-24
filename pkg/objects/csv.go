package objects

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// _csv is a C accelerator module in CPython, so the runtime provides it in Go.
// It carries the whole capability the pure-Python csv module re-exports: the
// reader and writer objects, the Dialect type that validates and holds the
// format parameters, and the field-size limit. csv.py binds reader and writer at
// import time, so the entire surface has to exist at once for `import csv` to
// work at all.
//
// The parsing and formatting live here in pkg/objects, next to the object types
// they act on, the same split array.array uses: the type and its machinery are
// here, and pkg/runtime registers the module and the callables. The dialect,
// reader and writer are native Go object types rather than Python classes so the
// reader can hold its input iterator and per-row parse state directly.

// The quoting styles, matching _csv's integer constants exactly.
const (
	csvQuoteMinimal    = 0
	csvQuoteAll        = 1
	csvQuoteNonnumeric = 2
	csvQuoteNone       = 3
	csvQuoteStrings    = 4
	csvQuoteNotnull    = 5
)

// csvNotSet marks a quotechar or escapechar left unset; no real rune equals it,
// so the parser's char comparisons never match an unset character. csvEOL is the
// end-of-line sentinel the reader feeds after each source line, standing in for
// _csv's EOL so a record can close without a trailing newline in the data.
const (
	csvNotSet = rune(-1)
	csvEOL    = rune(-2)
)

// csvFieldLimit is the module-wide cap on a single parsed field, _csv's
// field_limit. It starts at 128 KiB, the CPython default, and field_size_limit
// reads and writes it.
var csvFieldLimit int64 = 128 * 1024

// CsvFieldLimit returns the current field-size limit.
func CsvFieldLimit() int64 { return csvFieldLimit }

// CsvSetFieldLimit sets the field-size limit and returns the previous value, the
// field_size_limit(newlimit) contract.
func CsvSetFieldLimit(n int64) int64 {
	old := csvFieldLimit
	csvFieldLimit = n
	return old
}

// csvDialect is a validated set of CSV format parameters, _csv.Dialect. The
// characters are stored as runes, with csvNotSet standing for an absent
// quotechar or escapechar; every field has already passed the same validation
// CPython's dialect_new applies, so the reader and writer can trust it.
type csvDialect struct {
	delimiter        rune
	quotechar        rune
	escapechar       rune
	doublequote      bool
	skipinitialspace bool
	lineterminator   string
	quoting          int
	strict           bool
}

func (d *csvDialect) TypeName() string { return "_csv.Dialect" }

// csvCharOrNone renders a stored character back as a length-1 str, or None when
// it is unset, the way Dialect's delimiter, quotechar and escapechar getters do.
func csvCharOrNone(c rune) Object {
	if c == csvNotSet {
		return None
	}
	return NewStr(string(c))
}

// csvDialectAttr reads one of the eight dialect attributes as CPython's getset
// and member descriptors report them.
func csvDialectAttr(d *csvDialect, name string) (Object, error) {
	switch name {
	case "delimiter":
		return csvCharOrNone(d.delimiter), nil
	case "quotechar":
		return csvCharOrNone(d.quotechar), nil
	case "escapechar":
		return csvCharOrNone(d.escapechar), nil
	case "doublequote":
		return NewBool(d.doublequote), nil
	case "skipinitialspace":
		return NewBool(d.skipinitialspace), nil
	case "lineterminator":
		return NewStr(d.lineterminator), nil
	case "quoting":
		return NewInt(int64(d.quoting)), nil
	case "strict":
		return NewBool(d.strict), nil
	}
	return nil, Raise(AttributeError, "'_csv.Dialect' object has no attribute '%s'", name)
}

// csvDialectParamNames is the set of keyword names dialect construction accepts,
// the eight format parameters plus the base "dialect" itself.
var csvDialectParamNames = map[string]bool{
	"dialect": true, "delimiter": true, "doublequote": true, "escapechar": true,
	"lineterminator": true, "quotechar": true, "quoting": true,
	"skipinitialspace": true, "strict": true,
}

// NewCsvDialect builds a dialect from an optional base and format keywords,
// mirroring _csv's dialect_new. base may be nil (all defaults), a registered
// dialect object already resolved from a name by the caller, or any object whose
// attributes name the parameters (a csv.py Dialect class or instance). Keywords
// override the base. A base that is already a dialect and needs no override is
// returned unchanged, the reuse the C code performs.
func NewCsvDialect(base Object, kwNames []string, kwVals []Object) (Object, error) {
	for _, k := range kwNames {
		if !csvDialectParamNames[k] {
			return nil, Raise(TypeError, "this function got an unexpected keyword argument '%s'", k)
		}
	}
	if bd, ok := base.(*csvDialect); ok && len(kwNames) == 0 {
		return bd, nil
	}

	// get resolves a parameter to its supplied value: a keyword wins, then a
	// base attribute, and a missing base attribute reads as absent so the
	// default applies. A base attribute that raises is treated as absent, the
	// way dialect_new clears the error.
	get := func(name string) Object {
		for i, k := range kwNames {
			if k == name {
				return kwVals[i]
			}
		}
		if base != nil {
			if v, err := LoadAttr(base, name); err == nil {
				return v
			}
		}
		return nil
	}

	quotecharObj := get("quotechar")
	quotingObj := get("quoting")

	d := &csvDialect{}
	var err error
	if d.delimiter, err = csvSetChar("delimiter", get("delimiter"), ','); err != nil {
		return nil, err
	}
	if d.doublequote, err = csvSetBool(get("doublequote"), true); err != nil {
		return nil, err
	}
	if d.escapechar, err = csvSetCharOrNone("escapechar", get("escapechar"), csvNotSet); err != nil {
		return nil, err
	}
	if d.lineterminator, err = csvSetStr("lineterminator", get("lineterminator"), "\r\n"); err != nil {
		return nil, err
	}
	if d.quotechar, err = csvSetCharOrNone("quotechar", quotecharObj, '"'); err != nil {
		return nil, err
	}
	if d.quoting, err = csvSetInt("quoting", quotingObj, csvQuoteMinimal); err != nil {
		return nil, err
	}
	if d.skipinitialspace, err = csvSetBool(get("skipinitialspace"), false); err != nil {
		return nil, err
	}
	if d.strict, err = csvSetBool(get("strict"), false); err != nil {
		return nil, err
	}

	if !csvValidQuoting(d.quoting) {
		return nil, Raise(TypeError, "bad \"quoting\" value")
	}
	// quotechar=None with no explicit quoting means quoting is off.
	if quotecharObj == None && quotingObj == nil {
		d.quoting = csvQuoteNone
	}
	if d.quoting != csvQuoteNone && d.quotechar == csvNotSet {
		return nil, Raise(TypeError, "quotechar must be set if quoting enabled")
	}
	if err := csvCheckChar("delimiter", d.delimiter, d, true); err != nil {
		return nil, err
	}
	if err := csvCheckChar("escapechar", d.escapechar, d, !d.skipinitialspace); err != nil {
		return nil, err
	}
	if err := csvCheckChar("quotechar", d.quotechar, d, !d.skipinitialspace); err != nil {
		return nil, err
	}
	if err := csvCheckChars("delimiter", "escapechar", d.delimiter, d.escapechar); err != nil {
		return nil, err
	}
	if err := csvCheckChars("delimiter", "quotechar", d.delimiter, d.quotechar); err != nil {
		return nil, err
	}
	if err := csvCheckChars("escapechar", "quotechar", d.escapechar, d.quotechar); err != nil {
		return nil, err
	}
	return d, nil
}

func csvValidQuoting(q int) bool {
	return q >= csvQuoteMinimal && q <= csvQuoteNotnull
}

// csvSetChar validates a required single-character parameter, defaulting when
// absent, and spells the type and length errors CPython's _set_char gives.
func csvSetChar(name string, o Object, dflt rune) (rune, error) {
	if o == nil {
		return dflt, nil
	}
	s, ok := AsStr(o)
	if !ok {
		return 0, Raise(TypeError, "\"%s\" must be a unicode character, not %s", name, o.TypeName())
	}
	r := []rune(s)
	if len(r) != 1 {
		return 0, Raise(TypeError, "\"%s\" must be a unicode character, not a string of length %d", name, len(r))
	}
	return r[0], nil
}

// csvSetCharOrNone is csvSetChar for a parameter that also accepts None, which
// reads back as unset.
func csvSetCharOrNone(name string, o Object, dflt rune) (rune, error) {
	if o == nil {
		return dflt, nil
	}
	if o == None {
		return csvNotSet, nil
	}
	s, ok := AsStr(o)
	if !ok {
		return 0, Raise(TypeError, "\"%s\" must be a unicode character or None, not %s", name, o.TypeName())
	}
	r := []rune(s)
	if len(r) != 1 {
		return 0, Raise(TypeError, "\"%s\" must be a unicode character or None, not a string of length %d", name, len(r))
	}
	return r[0], nil
}

// csvSetBool coerces a parameter through truthiness, defaulting when absent.
func csvSetBool(o Object, dflt bool) (bool, error) {
	if o == nil {
		return dflt, nil
	}
	return Truth(o), nil
}

// csvSetInt reads an integer parameter, defaulting when absent. It requires an
// exact int, so a bool is rejected the way _set_int's PyLong_CheckExact does.
func csvSetInt(name string, o Object, dflt int) (int, error) {
	if o == nil {
		return dflt, nil
	}
	if _, isBool := o.(*boolObject); isBool {
		return 0, Raise(TypeError, "\"%s\" must be an integer, not bool", name)
	}
	if _, isInt := o.(*intObject); !isInt {
		return 0, Raise(TypeError, "\"%s\" must be an integer, not %s", name, o.TypeName())
	}
	v, _ := AsInt(o)
	return int(v), nil
}

// csvSetStr reads a string parameter, defaulting when absent.
func csvSetStr(name string, o Object, dflt string) (string, error) {
	if o == nil {
		return dflt, nil
	}
	s, ok := AsStr(o)
	if !ok {
		return "", Raise(TypeError, "\"%s\" must be a string, not %s", name, o.TypeName())
	}
	return s, nil
}

// csvCheckChar rejects a character that would confuse the parser: a carriage
// return or newline, a space where one is not allowed, or a character that also
// appears in the line terminator.
func csvCheckChar(name string, c rune, d *csvDialect, allowspace bool) error {
	if c == csvNotSet {
		return nil
	}
	if c == '\r' || c == '\n' || (c == ' ' && !allowspace) {
		return Raise(ValueError, "bad %s value", name)
	}
	if strings.ContainsRune(d.lineterminator, c) {
		return Raise(ValueError, "bad %s or lineterminator value", name)
	}
	return nil
}

// csvCheckChars rejects two set characters that collide, so the delimiter,
// escapechar and quotechar stay distinct.
func csvCheckChars(name1, name2 string, c1, c2 rune) error {
	if c1 == c2 && c1 != csvNotSet {
		return Raise(ValueError, "bad %s or %s value", name1, name2)
	}
	return nil
}

// The reader's per-character parse states, matching _csv's state enum.
const (
	csvStartRecord = iota
	csvStartField
	csvEscapedChar
	csvInField
	csvInQuotedField
	csvEscapeInQuotedField
	csvQuoteInQuotedField
	csvEatCRNL
	csvAfterEscapedCRNL
)

// csvReader parses lines pulled from an iterable into rows of string fields. It
// is its own iterator: iter(reader) is reader, so line_num stays visible across
// a loop and a second pass finds it exhausted.
type csvReader struct {
	input    Iterator
	dialect  *csvDialect
	errClass Object
	lineNum  int64

	// Per-record parse state, reset at the start of each row.
	fields        []Object
	field         []rune
	state         int
	unquotedField bool
}

func (r *csvReader) TypeName() string { return "_csv.reader" }

// Iterate returns the reader itself, the self-iterator contract.
func (r *csvReader) Iterate() (Iterator, error) { return r, nil }

// NewCsvReader builds a reader over an iterable under a dialect. The iterable is
// turned into an iterator immediately, so a non-iterable argument raises here the
// way _csv does at construction.
func NewCsvReader(iterable Object, dialect Object, errClass Object) (Object, error) {
	d, ok := dialect.(*csvDialect)
	if !ok {
		return nil, Raise(TypeError, "argument 2 must be a dialect")
	}
	it, err := Iter(iterable)
	if err != nil {
		return nil, err
	}
	return &csvReader{input: it, dialect: d, errClass: errClass}, nil
}

// csvReaderLoadAttr reads the reader's two data attributes: the row-counting
// line_num and the read-only dialect.
func csvReaderLoadAttr(r *csvReader, name string) (Object, error) {
	switch name {
	case "line_num":
		return NewInt(r.lineNum), nil
	case "dialect":
		return r.dialect, nil
	}
	return nil, Raise(AttributeError, "'_csv.reader' object has no attribute '%s'", name)
}

// resetRecord clears the per-row parse state before reading the next row.
func (r *csvReader) resetRecord() {
	r.fields = nil
	r.field = r.field[:0]
	r.state = csvStartRecord
	r.unquotedField = false
}

// Next reads and returns the next row as a list of string fields, or reports
// exhaustion. A quoted field can span several source lines, so it pulls from the
// input until a record closes.
func (r *csvReader) Next() (Object, bool, error) {
	r.resetRecord()
	for {
		lineObj, ok, err := r.input.Next()
		if err != nil {
			return nil, false, err
		}
		if !ok {
			// End of input: a field in progress or an open quote closes as a
			// final record unless strict mode rejects the truncation.
			if len(r.field) != 0 || r.state == csvInQuotedField {
				if r.dialect.strict {
					return nil, false, csvErrorf(r.errClass, "unexpected end of data")
				}
				if err := r.saveField(); err != nil {
					return nil, false, err
				}
				row := r.fields
				r.fields = nil
				return NewList(row), true, nil
			}
			return nil, false, nil
		}
		s, ok := AsStr(lineObj)
		if !ok {
			return nil, false, csvErrorf(r.errClass,
				"iterator should return strings, not %s (the file should be opened in text mode)",
				lineObj.TypeName())
		}
		r.lineNum++
		for _, c := range s {
			if err := r.processChar(c); err != nil {
				return nil, false, err
			}
		}
		if err := r.processChar(csvEOL); err != nil {
			return nil, false, err
		}
		if r.state == csvStartRecord {
			break
		}
	}
	row := r.fields
	r.fields = nil
	return NewList(row), true, nil
}

// addChar appends one character to the field in progress, enforcing the field
// size limit the way _csv's parse_add_char does.
func (r *csvReader) addChar(c rune) error {
	if int64(len(r.field)) >= csvFieldLimit {
		return csvErrorf(r.errClass, "field larger than field limit (%d)", csvFieldLimit)
	}
	r.field = append(r.field, c)
	return nil
}

// saveField commits the field in progress to the current row. An empty unquoted
// field reads as None under the NOTNULL and STRINGS styles, and a non-empty
// unquoted field is converted to a float under NONNUMERIC and STRINGS.
func (r *csvReader) saveField() error {
	q := r.dialect.quoting
	if r.unquotedField && len(r.field) == 0 && (q == csvQuoteNotnull || q == csvQuoteStrings) {
		r.fields = append(r.fields, None)
		return nil
	}
	s := string(r.field)
	var field Object = NewStr(s)
	if r.unquotedField && len(r.field) != 0 && (q == csvQuoteNonnumeric || q == csvQuoteStrings) {
		f, err := csvParseFloat(s)
		if err != nil {
			return err
		}
		field = f
	}
	r.field = r.field[:0]
	r.fields = append(r.fields, field)
	return nil
}

// processChar advances the state machine by one character, the direct Go
// transcription of _csv's parse_process_char.
func (r *csvReader) processChar(c rune) error {
	d := r.dialect
	switch r.state {
	case csvStartRecord:
		if c == csvEOL {
			return nil
		}
		if c == '\n' || c == '\r' {
			r.state = csvEatCRNL
			return nil
		}
		r.state = csvStartField
		fallthrough

	case csvStartField:
		r.unquotedField = true
		if c == '\n' || c == '\r' || c == csvEOL {
			if err := r.saveField(); err != nil {
				return err
			}
			r.state = csvEndState(c)
		} else if d.quotechar != csvNotSet && c == d.quotechar && d.quoting != csvQuoteNone {
			r.unquotedField = false
			r.state = csvInQuotedField
		} else if d.escapechar != csvNotSet && c == d.escapechar {
			r.state = csvEscapedChar
		} else if c == ' ' && d.skipinitialspace {
			// ignore leading spaces
		} else if c == d.delimiter {
			if err := r.saveField(); err != nil {
				return err
			}
		} else {
			if err := r.addChar(c); err != nil {
				return err
			}
			r.state = csvInField
		}

	case csvEscapedChar:
		if c == '\n' || c == '\r' {
			if err := r.addChar(c); err != nil {
				return err
			}
			r.state = csvAfterEscapedCRNL
			return nil
		}
		if c == csvEOL {
			c = '\n'
		}
		if err := r.addChar(c); err != nil {
			return err
		}
		r.state = csvInField

	case csvAfterEscapedCRNL:
		if c == csvEOL {
			return nil
		}
		fallthrough

	case csvInField:
		if c == '\n' || c == '\r' || c == csvEOL {
			if err := r.saveField(); err != nil {
				return err
			}
			r.state = csvEndState(c)
		} else if d.escapechar != csvNotSet && c == d.escapechar {
			r.state = csvEscapedChar
		} else if c == d.delimiter {
			if err := r.saveField(); err != nil {
				return err
			}
			r.state = csvStartField
		} else {
			if err := r.addChar(c); err != nil {
				return err
			}
		}

	case csvInQuotedField:
		if c == csvEOL {
			// stay in the quoted field, spanning to the next line
		} else if d.escapechar != csvNotSet && c == d.escapechar {
			r.state = csvEscapeInQuotedField
		} else if d.quotechar != csvNotSet && c == d.quotechar && d.quoting != csvQuoteNone {
			if d.doublequote {
				r.state = csvQuoteInQuotedField
			} else {
				r.state = csvInField
			}
		} else {
			if err := r.addChar(c); err != nil {
				return err
			}
		}

	case csvEscapeInQuotedField:
		if c == csvEOL {
			c = '\n'
		}
		if err := r.addChar(c); err != nil {
			return err
		}
		r.state = csvInQuotedField

	case csvQuoteInQuotedField:
		if d.quoting != csvQuoteNone && d.quotechar != csvNotSet && c == d.quotechar {
			if err := r.addChar(c); err != nil {
				return err
			}
			r.state = csvInQuotedField
		} else if c == d.delimiter {
			if err := r.saveField(); err != nil {
				return err
			}
			r.state = csvStartField
		} else if c == '\n' || c == '\r' || c == csvEOL {
			if err := r.saveField(); err != nil {
				return err
			}
			r.state = csvEndState(c)
		} else if !d.strict {
			if err := r.addChar(c); err != nil {
				return err
			}
			r.state = csvInField
		} else {
			return csvErrorf(r.errClass, "'%c' expected after '%c'", d.delimiter, d.quotechar)
		}

	case csvEatCRNL:
		if c == '\n' || c == '\r' {
			// swallow the rest of the line ending
		} else if c == csvEOL {
			r.state = csvStartRecord
		} else {
			return csvErrorf(r.errClass,
				"new-line character seen in unquoted field - do you need to open the file with newline=''?")
		}
	}
	return nil
}

// csvEndState picks the state after a field closes at a line ending: a record
// completes on the end-of-line sentinel, otherwise the reader eats the rest of
// the carriage-return/newline sequence.
func csvEndState(c rune) int {
	if c == csvEOL {
		return csvStartRecord
	}
	return csvEatCRNL
}

// csvParseFloat converts an unquoted field to a float under the numeric quoting
// styles, spelling the ValueError float() gives for an unconvertible string. It
// tracks the project's float() string handling: surrounding whitespace is
// ignored, C hex-float syntax is rejected, and an out-of-range magnitude becomes
// an infinity rather than an error.
func csvParseFloat(s string) (Object, error) {
	trimmed := strings.TrimFunc(s, unicode.IsSpace)
	bad := trimmed == ""
	lower := strings.ToLower(strings.TrimLeft(trimmed, "+-"))
	if strings.HasPrefix(lower, "0x") {
		bad = true
	}
	var v float64
	if !bad {
		f, err := strconv.ParseFloat(trimmed, 64)
		if err != nil && !strings.Contains(err.Error(), "out of range") {
			bad = true
		}
		v = f
	}
	if bad {
		return nil, Raise(ValueError, "could not convert string to float: %s", Repr(NewStr(s)))
	}
	return NewFloat(v), nil
}

// csvWriter formats rows and writes them to a file object, _csv.writer. It holds
// the file's bound write callable and a dialect, and exposes writerow, writerows
// and the read-only dialect.
type csvWriter struct {
	write    Object
	dialect  *csvDialect
	errClass Object
}

func (w *csvWriter) TypeName() string { return "_csv.writer" }

// NewCsvWriter builds a writer over a file object under a dialect. The file must
// expose a callable write, the one method _csv requires.
func NewCsvWriter(fileobj Object, dialect Object, errClass Object) (Object, error) {
	d, ok := dialect.(*csvDialect)
	if !ok {
		return nil, Raise(TypeError, "argument 2 must be a dialect")
	}
	write, err := LoadAttr(fileobj, "write")
	if err != nil || !Callable(write) {
		return nil, Raise(TypeError, "argument 1 must have a \"write\" method")
	}
	return &csvWriter{write: write, dialect: d, errClass: errClass}, nil
}

// csvWriterLoadAttr reads the writer's dialect attribute or binds writerow and
// writerows as callables, so w.writerow reads back and calls the same as
// w.writerow(row).
func csvWriterLoadAttr(w *csvWriter, name string) (Object, error) {
	switch name {
	case "dialect":
		return w.dialect, nil
	case "writerow", "writerows":
		return builtinMethodValue(w, name), nil
	}
	return nil, Raise(AttributeError, "'_csv.writer' object has no attribute '%s'", name)
}

// csvWriterMethod dispatches w.writerow(row) and w.writerows(rows).
func csvWriterMethod(w *csvWriter, name string, args []Object) (Object, error) {
	switch name {
	case "writerow":
		if len(args) != 1 {
			return nil, Raise(TypeError, "writerow() takes exactly 1 argument (%d given)", len(args))
		}
		return w.writeRow(args[0])
	case "writerows":
		if len(args) != 1 {
			return nil, Raise(TypeError, "writerows() takes exactly 1 argument (%d given)", len(args))
		}
		return w.writeRows(args[0])
	}
	return nil, noAttr(w, name)
}

// writeRow formats one row and hands the joined line to the file's write,
// returning whatever write returns (a character count for a text file). Each
// field is quoted according to the dialect's style before joining.
func (w *csvWriter) writeRow(seq Object) (Object, error) {
	it, err := Iter(seq)
	if err != nil {
		return nil, csvErrorf(w.errClass, "iterable expected, not %s", seq.TypeName())
	}
	d := w.dialect
	var rec []rune
	numFields := 0
	nullField := false
	for {
		field, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		quoted := false
		switch d.quoting {
		case csvQuoteNonnumeric:
			quoted = !csvIsNumber(field)
		case csvQuoteAll:
			quoted = true
		case csvQuoteStrings:
			_, quoted = field.(*strObject)
		case csvQuoteNotnull:
			quoted = field != None
		}
		nullField = field == None

		var text *string
		if s, ok := field.(*strObject); ok {
			v := s.v
			text = &v
		} else if nullField {
			text = nil
		} else {
			sv, err := StrE(field)
			if err != nil {
				return nil, err
			}
			text = &sv
		}
		if err := w.joinAppend(&rec, text, quoted, numFields); err != nil {
			return nil, err
		}
		numFields++
	}

	// A lone empty field leaves an empty record; CPython forces it to be quoted
	// so the row is not lost, and rejects the styles that cannot represent it.
	if numFields > 0 && len(rec) == 0 {
		if d.quoting == csvQuoteNone ||
			(nullField && (d.quoting == csvQuoteStrings || d.quoting == csvQuoteNotnull)) {
			return nil, csvErrorf(w.errClass, "single empty field record must be quoted")
		}
		if err := w.joinAppend(&rec, nil, true, 0); err != nil {
			return nil, err
		}
	}
	rec = append(rec, []rune(d.lineterminator)...)
	return Call(w.write, []Object{NewStr(string(rec))})
}

// writeRows formats and writes every row of an iterable, returning None.
func (w *csvWriter) writeRows(seq Object) (Object, error) {
	it, err := Iter(seq)
	if err != nil {
		return nil, err
	}
	for {
		row, ok, err := it.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		if _, err := w.writeRow(row); err != nil {
			return nil, err
		}
	}
	return None, nil
}

// joinAppend appends one formatted field to the record buffer. It handles the
// space-delimiter special case, where an empty field would vanish, then defers
// the character-by-character escaping and quoting to joinAppendData.
func (w *csvWriter) joinAppend(rec *[]rune, field *string, quoted bool, numFields int) error {
	d := w.dialect
	fieldEmpty := field == nil || *field == ""
	if fieldEmpty && d.delimiter == ' ' && d.skipinitialspace {
		if d.quoting == csvQuoteNone ||
			(field == nil && (d.quoting == csvQuoteStrings || d.quoting == csvQuoteNotnull)) {
			return csvErrorf(w.errClass,
				"empty field must be quoted if delimiter is a space and skipinitialspace is true")
		}
		quoted = true
	}
	return w.joinAppendData(rec, field, quoted, numFields)
}

// joinAppendData writes one field into the record: a leading delimiter when it
// is not the first field, then the field's characters with the dialect's
// escaping, wrapped in quotes when quoting is called for. It is the Go form of
// _csv's join_append_data with the count and copy phases folded into one pass,
// since the final quoted flag is known before the wrapping quotes are written.
func (w *csvWriter) joinAppendData(rec *[]rune, field *string, quoted bool, numFields int) error {
	d := w.dialect
	var inner []rune
	if field != nil {
		for _, c := range *field {
			special := c == d.delimiter ||
				(d.escapechar != csvNotSet && c == d.escapechar) ||
				(d.quotechar != csvNotSet && c == d.quotechar) ||
				c == '\n' || c == '\r' ||
				strings.ContainsRune(d.lineterminator, c)
			if special {
				wantEscape := false
				if d.quoting == csvQuoteNone {
					wantEscape = true
				} else {
					if d.quotechar != csvNotSet && c == d.quotechar {
						if d.doublequote {
							inner = append(inner, d.quotechar)
						} else {
							wantEscape = true
						}
					} else if d.escapechar != csvNotSet && c == d.escapechar {
						wantEscape = true
					}
					if !wantEscape {
						quoted = true
					}
				}
				if wantEscape {
					if d.escapechar == csvNotSet {
						return csvErrorf(w.errClass, "need to escape, but no escapechar set")
					}
					inner = append(inner, d.escapechar)
				}
			}
			inner = append(inner, c)
		}
	}
	if numFields > 0 {
		*rec = append(*rec, d.delimiter)
	}
	if quoted {
		*rec = append(*rec, d.quotechar)
	}
	*rec = append(*rec, inner...)
	if quoted {
		*rec = append(*rec, d.quotechar)
	}
	return nil
}

// csvIsNumber reports whether a field counts as numeric for QUOTE_NONNUMERIC,
// matching PyNumber_Check over the builtin numbers: int, bool, float and
// complex.
func csvIsNumber(o Object) bool {
	switch o.(type) {
	case *intObject, *boolObject, *floatObject, *complexObject:
		return true
	}
	return false
}

// csvErrorf raises a _csv.Error carrying the formatted message, the exception the
// reader, writer and Sniffer raise and callers catch with `except csv.Error`.
// The error class is built by the runtime module and threaded onto each object.
func csvErrorf(errClass Object, format string, a ...any) error {
	msg := fmt.Sprintf(format, a...)
	if errClass != nil {
		if inst, err := Call(errClass, []Object{NewStr(msg)}); err == nil {
			if e, ok := inst.(error); ok {
				return e
			}
		}
	}
	return Raise(RuntimeError, "%s", msg)
}
