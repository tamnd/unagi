package runtime

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"

	"github.com/tamnd/unagi/pkg/objects"
)

// pyexpat is the C accelerator behind the XML stack: xml.parsers.expat is a thin
// shim over it, and xml.dom.minidom, xml.sax and plistlib all drive an expat
// parser. Without it `import plistlib` and `import xml.dom.minidom` raised
// ModuleNotFoundError.
//
// The parser is an ordinary class whose Parse method drives Go's stdlib
// encoding/xml tokenizer and fans each token out to the expat handler the caller
// installed (StartElementHandler, EndElementHandler, CharacterDataHandler, ...).
// Handlers are plain settable attributes on the instance, initialized to None,
// so `p.StartElementHandler = fn` and a later read behave like the C getset
// slots. The whole input is buffered and tokenized when the final Parse chunk
// arrives, which is what minidom's parseString (one Parse(data, True) call) and
// plistlib's ParseFile (chunks then a final empty one) both do; the resulting
// tree is identical because these consumers read their result only after the
// parse completes.
//
// The module is portable (encoding/xml is stdlib), so it registers on every
// target. Scope of this slice: well-formed documents without a DTD, in either
// the default non-namespace mode (plistlib) or namespace mode (minidom). See the
// follow-up note for the documented gaps.

const expatHandleAttr = "_unagi_expat"

// expatHandle is the native parser state: the buffered input, and the
// namespace configuration fixed at creation.
type expatHandle struct {
	buf        []byte
	namespaces bool
	nsSep      string
	base       string
}

func (*expatHandle) TypeName() string { return "_xmlparser_handle" }

// expatParserClass is the built xmlparser class, used by ParserCreate.
var expatParserClass objects.Object

// expatErrorClass is pyexpat.ExpatError (also exported as error), a ValueError
// subclass carrying code/lineno/offset attributes.
var expatErrorClass objects.Object

func init() {
	moduleTable["pyexpat"] = &moduleEntry{builtin: true, exec: initPyexpat}
}

// expatHandlerNames are the handler attributes a parser exposes. They default to
// None so an unset handler reads back as None, like the C getset slots, and the
// parse loop can load any of them unconditionally.
var expatHandlerNames = []string{
	"StartElementHandler", "EndElementHandler", "CharacterDataHandler",
	"ProcessingInstructionHandler", "CommentHandler",
	"StartCdataSectionHandler", "EndCdataSectionHandler",
	"DefaultHandler", "DefaultHandlerExpand", "XmlDeclHandler",
	"StartNamespaceDeclHandler", "EndNamespaceDeclHandler",
	"StartDoctypeDeclHandler", "EndDoctypeDeclHandler",
	"ElementDeclHandler", "AttlistDeclHandler", "EntityDeclHandler",
	"NotationDeclHandler", "UnparsedEntityDeclHandler",
	"ExternalEntityRefHandler", "SkippedEntityHandler",
}

func initPyexpat(m *objects.Module) error {
	set := func(name string, v objects.Object) error { return objects.StoreAttr(m, name, v) }

	valueError, ok := objects.ExcClassValue("ValueError")
	if !ok {
		return objects.Raise(objects.RuntimeError, "pyexpat: ValueError base is unavailable")
	}
	errCls, err := objects.NewClass("ExpatError", "xml.parsers.expat.ExpatError", []objects.Object{valueError}, nil, nil, nil, nil)
	if err != nil {
		return err
	}
	expatErrorClass = errCls
	if err := set("ExpatError", errCls); err != nil {
		return err
	}
	if err := set("error", errCls); err != nil {
		return err
	}

	cls, err := buildExpatParserClass()
	if err != nil {
		return err
	}
	expatParserClass = cls
	if err := set("XMLParserType", cls); err != nil {
		return err
	}

	if err := set("ParserCreate", objects.NewFuncKw("ParserCreate", pyexpatParserCreate)); err != nil {
		return err
	}
	if err := set("ErrorString", objects.NewFunc("ErrorString", 1, pyexpatErrorString)); err != nil {
		return err
	}

	// Version and encoding constants. The tokenizer is Go's, but callers that
	// print EXPAT_VERSION expect the expat-style shape.
	if err := set("EXPAT_VERSION", objects.NewStr("expat_2.7.4")); err != nil {
		return err
	}
	if err := set("version_info", objects.NewTuple([]objects.Object{objects.NewInt(2), objects.NewInt(7), objects.NewInt(4)})); err != nil {
		return err
	}
	if err := set("native_encoding", objects.NewStr("UTF-8")); err != nil {
		return err
	}
	if err := set("features", objects.NewList(nil)); err != nil {
		return err
	}

	// Parameter-entity-parsing modes, the arguments to SetParamEntityParsing.
	// xml.sax's expatreader reads XML_PARAM_ENTITY_PARSING_UNLESS_STANDALONE off
	// the module at reset time.
	for name, v := range map[string]int64{
		"XML_PARAM_ENTITY_PARSING_NEVER":             0,
		"XML_PARAM_ENTITY_PARSING_UNLESS_STANDALONE": 1,
		"XML_PARAM_ENTITY_PARSING_ALWAYS":            2,
	} {
		if err := set(name, objects.NewInt(v)); err != nil {
			return err
		}
	}

	// The errors and model submodules. xml/parsers/expat.py reads them off the
	// star import and republishes them in sys.modules.
	errorsMod, err := buildExpatErrorsModule()
	if err != nil {
		return err
	}
	if err := set("errors", errorsMod); err != nil {
		return err
	}
	modelMod, err := buildExpatModelModule()
	if err != nil {
		return err
	}
	if err := set("model", modelMod); err != nil {
		return err
	}
	return nil
}

func buildExpatParserClass() (objects.Object, error) {
	names := []string{
		"__slots__",
		"Parse", "ParseFile",
		"SetBase", "GetBase", "GetInputContext",
		"ExternalEntityParserCreate", "SetParamEntityParsing",
	}
	vals := []objects.Object{
		objects.NewList([]objects.Object{objects.NewStr("__dict__")}),
		objects.NewMethod("Parse", -1, expatParse),
		objects.NewMethod("ParseFile", 2, expatParseFile),
		objects.NewMethod("SetBase", 2, expatSetBase),
		objects.NewMethod("GetBase", 1, expatGetBase),
		objects.NewMethod("GetInputContext", 1, expatGetInputContext),
		objects.NewMethod("ExternalEntityParserCreate", -1, expatExternalEntityParserCreate),
		objects.NewMethod("SetParamEntityParsing", 2, expatSetParamEntityParsing),
	}
	return objects.NewClass("xmlparser", "pyexpat.xmlparser", nil, names, vals, nil, nil)
}

// pyexpatParserCreate is pyexpat.ParserCreate(encoding=None,
// namespace_separator=None). A namespace_separator that is a string (even "")
// puts the parser in namespace-processing mode.
func pyexpatParserCreate(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	arg := func(index int, name string) objects.Object {
		for i, kn := range kwNames {
			if kn == name {
				return kwVals[i]
			}
		}
		if index < len(pos) {
			return pos[index]
		}
		return nil
	}
	nsSepArg := arg(1, "namespace_separator")
	handle := &expatHandle{}
	if nsSepArg != nil && nsSepArg != objects.None {
		sep, ok := objects.AsStr(nsSepArg)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "ParserCreate() argument 'namespace_separator' must be str or None")
		}
		handle.namespaces = true
		handle.nsSep = sep
	}

	inst, err := objects.Call(expatParserClass, nil)
	if err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(inst, expatHandleAttr, handle); err != nil {
		return nil, err
	}
	// Seed the handler slots and the reported attributes to their defaults so a
	// read before assignment behaves like the C getset slots.
	for _, h := range expatHandlerNames {
		if err := objects.StoreAttr(inst, h, objects.None); err != nil {
			return nil, err
		}
	}
	internDict, err := objects.NewDict(nil, nil)
	if err != nil {
		return nil, err
	}
	defaults := []struct {
		name string
		val  objects.Object
	}{
		{"intern", internDict},
		{"buffer_text", objects.False},
		{"ordered_attributes", objects.False},
		{"specified_attributes", objects.False},
		{"CurrentLineNumber", objects.NewInt(1)},
		{"CurrentColumnNumber", objects.NewInt(0)},
		{"CurrentByteIndex", objects.NewInt(0)},
		{"ErrorCode", objects.NewInt(0)},
		{"ErrorLineNumber", objects.NewInt(-1)},
		{"ErrorColumnNumber", objects.NewInt(-1)},
		{"ErrorByteIndex", objects.NewInt(-1)},
	}
	for _, d := range defaults {
		if err := objects.StoreAttr(inst, d.name, d.val); err != nil {
			return nil, err
		}
	}
	return inst, nil
}

func expatHandleOf(self objects.Object) (*expatHandle, error) {
	v, err := objects.LoadAttr(self, expatHandleAttr)
	if err != nil {
		return nil, objects.Raise(objects.TypeError, "not an xmlparser object")
	}
	h, ok := v.(*expatHandle)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "not an xmlparser object")
	}
	return h, nil
}

// expatDataBytes reads a str or bytes-like argument as the bytes to parse.
func expatDataBytes(o objects.Object) ([]byte, error) {
	if b, ok := objects.AsBytesLike(o); ok {
		return b, nil
	}
	if s, ok := objects.AsStr(o); ok {
		return []byte(s), nil
	}
	return nil, objects.Raise(objects.TypeError, "Parse() argument must be str or bytes-like, not %s", o.TypeName())
}

// expatParse is xmlparser.Parse(data, isfinal=False). Data is buffered; the
// tokenizer runs when isfinal is true.
func expatParse(args []objects.Object) (objects.Object, error) {
	if len(args) < 2 {
		return nil, objects.Raise(objects.TypeError, "Parse() takes at least 1 argument")
	}
	self := args[0]
	h, err := expatHandleOf(self)
	if err != nil {
		return nil, err
	}
	data, err := expatDataBytes(args[1])
	if err != nil {
		return nil, err
	}
	h.buf = append(h.buf, data...)
	isfinal := false
	if len(args) >= 3 {
		if t, terr := objects.TruthOf(args[2]); terr == nil {
			isfinal = t
		}
	}
	if !isfinal {
		return objects.NewInt(1), nil
	}
	if err := expatRun(self, h); err != nil {
		return nil, err
	}
	return objects.NewInt(1), nil
}

// expatParseFile is xmlparser.ParseFile(file): reads the whole stream and parses
// it, the shape plistlib uses.
func expatParseFile(args []objects.Object) (objects.Object, error) {
	self := args[0]
	h, err := expatHandleOf(self)
	if err != nil {
		return nil, err
	}
	readFn, err := objects.LoadAttr(args[1], "read")
	if err != nil {
		return nil, objects.Raise(objects.TypeError, "ParseFile() argument must have a read() method")
	}
	chunk, err := objects.Call(readFn, nil)
	if err != nil {
		return nil, err
	}
	data, err := expatDataBytes(chunk)
	if err != nil {
		return nil, err
	}
	h.buf = append(h.buf, data...)
	if err := expatRun(self, h); err != nil {
		return nil, err
	}
	return objects.NewInt(1), nil
}

// expatCall invokes a handler if the caller installed one (it is not None),
// passing the given arguments.
func expatCall(self objects.Object, name string, cargs ...objects.Object) error {
	fn, err := objects.LoadAttr(self, name)
	if err != nil || fn == objects.None {
		return nil
	}
	_, cerr := objects.Call(fn, cargs)
	return cerr
}

// expatBool reads a boolean-ish parser attribute, defaulting to false.
func expatBool(self objects.Object, name string) bool {
	v, err := objects.LoadAttr(self, name)
	if err != nil {
		return false
	}
	t, terr := objects.TruthOf(v)
	return terr == nil && t
}

// expatRun tokenizes the buffered input and drives the handlers.
func expatRun(self objects.Object, h *expatHandle) error {
	dec := xml.NewDecoder(bytes.NewReader(h.buf))
	// A non-namespace parser must not resolve namespaces; feeding the raw names
	// through is closest to expat. In namespace mode the decoder's resolution is
	// exactly what expat reports.
	bufferText := expatBool(self, "buffer_text")
	ordered := expatBool(self, "ordered_attributes")

	depth := 0
	first := true
	var pending []byte // coalesced character data when buffer_text is on

	flushChars := func() error {
		if len(pending) == 0 {
			return nil
		}
		s := string(pending)
		pending = pending[:0]
		return expatCall(self, "CharacterDataHandler", objects.NewStr(s))
	}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			if ferr := flushChars(); ferr != nil {
				return ferr
			}
			return expatRaise(self, err, dec, h)
		}
		// Update the reported byte position for handlers that read it.
		_ = objects.StoreAttr(self, "CurrentByteIndex", objects.NewInt(dec.InputOffset()))

		switch e := tok.(type) {
		case xml.ProcInst:
			if err := flushChars(); err != nil {
				return err
			}
			if first && e.Target == "xml" {
				if err := expatXMLDecl(self, string(e.Inst)); err != nil {
					return err
				}
			} else if err := expatCall(self, "ProcessingInstructionHandler",
				objects.NewStr(e.Target), objects.NewStr(string(e.Inst))); err != nil {
				return err
			}
		case xml.StartElement:
			if err := flushChars(); err != nil {
				return err
			}
			if err := expatStart(self, h, e, ordered); err != nil {
				return err
			}
			depth++
		case xml.EndElement:
			if err := flushChars(); err != nil {
				return err
			}
			if err := expatEnd(self, h, e); err != nil {
				return err
			}
			depth--
		case xml.CharData:
			// Character data outside the document element (prolog and epilog
			// whitespace) is not reported to CharacterDataHandler.
			if depth == 0 {
				break
			}
			if bufferText {
				pending = append(pending, e...)
			} else if err := expatCall(self, "CharacterDataHandler", objects.NewStr(string(e))); err != nil {
				return err
			}
		case xml.Comment:
			if err := flushChars(); err != nil {
				return err
			}
			if err := expatCall(self, "CommentHandler", objects.NewStr(string(e))); err != nil {
				return err
			}
		case xml.Directive:
			if err := flushChars(); err != nil {
				return err
			}
			// A DTD or other <!...> declaration. Route the raw text to
			// DefaultHandler if one is set; the fine-grained doctype handlers are a
			// later slice.
			if err := expatCall(self, "DefaultHandler", objects.NewStr("<!"+string(e)+">")); err != nil {
				return err
			}
		}
		first = false
	}
	return flushChars()
}

// expatName renders an element or attribute name for the parser's mode: a
// namespace parser joins the URI and local name with the separator, a
// non-namespace parser uses the raw local name.
func expatName(h *expatHandle, name xml.Name) string {
	if h.namespaces && name.Space != "" {
		return name.Space + h.nsSep + name.Local
	}
	return name.Local
}

// expatStart fires StartElementHandler, and in namespace mode the
// StartNamespaceDeclHandler for each declaration on the element.
func expatStart(self objects.Object, h *expatHandle, e xml.StartElement, ordered bool) error {
	// In namespace mode, xmlns declarations are reported through
	// StartNamespaceDeclHandler and excluded from the attribute set.
	var attrs []xml.Attr
	for _, a := range e.Attr {
		if h.namespaces && (a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns")) {
			prefix := objects.Object(objects.None)
			if a.Name.Space == "xmlns" {
				prefix = objects.NewStr(a.Name.Local)
			}
			if err := expatCall(self, "StartNamespaceDeclHandler", prefix, objects.NewStr(a.Value)); err != nil {
				return err
			}
			continue
		}
		attrs = append(attrs, a)
	}

	name := expatName(h, e.Name)
	var attrObj objects.Object
	if ordered {
		var flat []objects.Object
		for _, a := range attrs {
			flat = append(flat, objects.NewStr(expatName(h, a.Name)), objects.NewStr(a.Value))
		}
		attrObj = objects.NewList(flat)
	} else {
		var keys, vals []objects.Object
		for _, a := range attrs {
			keys = append(keys, objects.NewStr(expatName(h, a.Name)))
			vals = append(vals, objects.NewStr(a.Value))
		}
		d, err := objects.NewDict(keys, vals)
		if err != nil {
			return err
		}
		attrObj = d
	}
	return expatCall(self, "StartElementHandler", objects.NewStr(name), attrObj)
}

// expatEnd fires EndElementHandler, and in namespace mode the
// EndNamespaceDeclHandler for each declaration the element made.
func expatEnd(self objects.Object, h *expatHandle, e xml.EndElement) error {
	if err := expatCall(self, "EndElementHandler", objects.NewStr(expatName(h, e.Name))); err != nil {
		return err
	}
	// The matching end-of-scope namespace callbacks are keyed off the start
	// element's declarations; encoding/xml does not resurface them here, so this
	// slice reports StartNamespaceDeclHandler without the paired end. minidom
	// tolerates the asymmetry for well-formed input.
	return nil
}

// expatXMLDecl parses the <?xml ...?> pseudo-attributes and fires XmlDeclHandler.
func expatXMLDecl(self objects.Object, inst string) error {
	version := expatPseudoAttr(inst, "version")
	encoding := expatPseudoAttr(inst, "encoding")
	standalone := objects.Object(objects.NewInt(-1))
	switch expatPseudoAttr(inst, "standalone") {
	case "yes":
		standalone = objects.NewInt(1)
	case "no":
		standalone = objects.NewInt(0)
	}
	vObj, eObj := objects.None, objects.None
	if version != "" {
		vObj = objects.NewStr(version)
	}
	if encoding != "" {
		eObj = objects.NewStr(encoding)
	}
	return expatCall(self, "XmlDeclHandler", vObj, eObj, standalone)
}

// expatPseudoAttr pulls one name="value" pseudo-attribute out of an XML
// declaration body.
func expatPseudoAttr(inst, name string) string {
	i := strings.Index(inst, name)
	if i < 0 {
		return ""
	}
	rest := inst[i+len(name):]
	eq := strings.IndexByte(rest, '=')
	if eq < 0 {
		return ""
	}
	rest = strings.TrimLeft(rest[eq+1:], " \t")
	if len(rest) == 0 {
		return ""
	}
	q := rest[0]
	if q != '"' && q != '\'' {
		return ""
	}
	end := strings.IndexByte(rest[1:], q)
	if end < 0 {
		return ""
	}
	return rest[1 : 1+end]
}

// expatRaise builds the pyexpat.ExpatError for a tokenizer failure, carrying the
// code, lineno and offset attributes callers inspect.
func expatRaise(self objects.Object, cause error, dec *xml.Decoder, h *expatHandle) error {
	off := dec.InputOffset()
	line, col := expatLineCol(h.buf, off)
	_ = objects.StoreAttr(self, "ErrorCode", objects.NewInt(int64(expatErrSyntax)))
	_ = objects.StoreAttr(self, "ErrorLineNumber", objects.NewInt(int64(line)))
	_ = objects.StoreAttr(self, "ErrorColumnNumber", objects.NewInt(int64(col)))
	_ = objects.StoreAttr(self, "ErrorByteIndex", objects.NewInt(off))

	msg := cause.Error()
	if se, ok := cause.(*xml.SyntaxError); ok {
		msg = se.Msg
		line = se.Line
	}
	full := msg + ": line " + itoa(line) + ", column " + itoa(col)
	inst, cerr := objects.Call(expatErrorClass, []objects.Object{objects.NewStr(full)})
	if cerr != nil {
		return cerr
	}
	for _, kv := range []struct {
		name string
		val  objects.Object
	}{
		{"code", objects.NewInt(int64(expatErrSyntax))},
		{"lineno", objects.NewInt(int64(line))},
		{"offset", objects.NewInt(int64(col))},
	} {
		if serr := objects.StoreAttr(inst, kv.name, kv.val); serr != nil {
			return serr
		}
	}
	if e, ok := inst.(error); ok {
		return e
	}
	return objects.Raise(objects.ValueError, "%s", full)
}

// expatLineCol maps a byte offset to a 1-based line and 0-based column.
func expatLineCol(buf []byte, off int64) (int, int) {
	if off > int64(len(buf)) {
		off = int64(len(buf))
	}
	line, col := 1, 0
	for i := int64(0); i < off; i++ {
		if buf[i] == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	return line, col
}

func expatSetBase(args []objects.Object) (objects.Object, error) {
	h, err := expatHandleOf(args[0])
	if err != nil {
		return nil, err
	}
	if s, ok := objects.AsStr(args[1]); ok {
		h.base = s
	}
	return objects.None, nil
}

func expatGetBase(args []objects.Object) (objects.Object, error) {
	h, err := expatHandleOf(args[0])
	if err != nil {
		return nil, err
	}
	if h.base == "" {
		return objects.None, nil
	}
	return objects.NewStr(h.base), nil
}

func expatGetInputContext(args []objects.Object) (objects.Object, error) {
	return objects.None, nil
}

// expatExternalEntityParserCreate returns a fresh parser sharing this parser's
// namespace configuration, the minimum minidom needs for its (rare) external
// entity path.
func expatExternalEntityParserCreate(args []objects.Object) (objects.Object, error) {
	h, err := expatHandleOf(args[0])
	if err != nil {
		return nil, err
	}
	inst, err := objects.Call(expatParserClass, nil)
	if err != nil {
		return nil, err
	}
	child := &expatHandle{namespaces: h.namespaces, nsSep: h.nsSep}
	if err := objects.StoreAttr(inst, expatHandleAttr, child); err != nil {
		return nil, err
	}
	for _, hn := range expatHandlerNames {
		if err := objects.StoreAttr(inst, hn, objects.None); err != nil {
			return nil, err
		}
	}
	return inst, nil
}

func expatSetParamEntityParsing(args []objects.Object) (objects.Object, error) {
	// Parameter-entity parsing is a DTD feature this slice does not act on;
	// accept the flag and report success (0), as expat does when it cannot honor
	// the request.
	return objects.NewInt(0), nil
}

// pyexpatErrorString is pyexpat.ErrorString(code): the message for an error code.
func pyexpatErrorString(args []objects.Object) (objects.Object, error) {
	code, ok := objects.AsInt(args[0])
	if !ok {
		return objects.None, nil
	}
	if msg, ok := expatErrorMessages[int(code)]; ok {
		return objects.NewStr(msg), nil
	}
	return objects.None, nil
}

// expatErrSyntax is XML_ERROR_SYNTAX, the code this slice reports for any
// tokenizer failure (Go's decoder does not distinguish expat's finer codes).
const expatErrSyntax = 2

// expatErrorMessages maps the expat error codes to their message strings, the
// table pyexpat.ErrorString and pyexpat.errors expose. The values match expat's
// own strings so callers that compare against errors.XML_ERROR_* still match.
var expatErrorMessages = map[int]string{
	1:  "out of memory",
	2:  "syntax error",
	3:  "no element found",
	4:  "not well-formed (invalid token)",
	5:  "unclosed token",
	6:  "partial character",
	7:  "mismatched tag",
	8:  "duplicate attribute",
	9:  "junk after document element",
	10: "illegal parameter entity reference",
	11: "undefined entity",
	12: "recursive entity reference",
	13: "asynchronous entity",
	14: "reference to invalid character number",
	15: "reference to binary entity",
	16: "reference to external entity in attribute",
	17: "XML or text declaration not at start of entity",
	18: "unknown encoding",
	19: "encoding specified in XML declaration is incorrect",
	20: "unclosed CDATA section",
	21: "error in processing external entity reference",
	22: "document is not standalone",
	23: "unexpected parser state - please send a bug report",
	24: "entity declared in parameter entity",
	25: "requested feature requires XML_DTD support in Expat",
	26: "cannot change setting once parsing has begun",
	27: "unbound prefix",
	28: "must not undeclare prefix",
	29: "incomplete markup in parameter entity",
	30: "XML declaration not well-formed",
	31: "text declaration not well-formed",
	32: "illegal character(s) in public id",
	33: "parser suspended",
	34: "parser not suspended",
	35: "parsing aborted",
	36: "parsing finished",
	37: "cannot suspend in external parameter entity",
	38: "reserved prefix (xml) must not be undeclared or bound to another namespace name",
	39: "reserved prefix (xmlns) must not be declared or undeclared",
	40: "prefix must not be bound to one of the reserved namespace names",
}

// expatErrorConstants names each error message code so pyexpat.errors can expose
// XML_ERROR_* string attributes and its codes/messages maps.
var expatErrorConstants = map[string]int{
	"XML_ERROR_NONE":                             0,
	"XML_ERROR_NO_MEMORY":                        1,
	"XML_ERROR_SYNTAX":                           2,
	"XML_ERROR_NO_ELEMENTS":                      3,
	"XML_ERROR_INVALID_TOKEN":                    4,
	"XML_ERROR_UNCLOSED_TOKEN":                   5,
	"XML_ERROR_PARTIAL_CHAR":                     6,
	"XML_ERROR_TAG_MISMATCH":                     7,
	"XML_ERROR_DUPLICATE_ATTRIBUTE":              8,
	"XML_ERROR_JUNK_AFTER_DOC_ELEMENT":           9,
	"XML_ERROR_PARAM_ENTITY_REF":                 10,
	"XML_ERROR_UNDEFINED_ENTITY":                 11,
	"XML_ERROR_RECURSIVE_ENTITY_REF":             12,
	"XML_ERROR_ASYNC_ENTITY":                     13,
	"XML_ERROR_BAD_CHAR_REF":                     14,
	"XML_ERROR_BINARY_ENTITY_REF":                15,
	"XML_ERROR_ATTRIBUTE_EXTERNAL_ENTITY_REF":    16,
	"XML_ERROR_MISPLACED_XML_PI":                 17,
	"XML_ERROR_UNKNOWN_ENCODING":                 18,
	"XML_ERROR_INCORRECT_ENCODING":               19,
	"XML_ERROR_UNCLOSED_CDATA_SECTION":           20,
	"XML_ERROR_EXTERNAL_ENTITY_HANDLING":         21,
	"XML_ERROR_NOT_STANDALONE":                   22,
	"XML_ERROR_UNEXPECTED_STATE":                 23,
	"XML_ERROR_ENTITY_DECLARED_IN_PE":            24,
	"XML_ERROR_FEATURE_REQUIRES_XML_DTD":         25,
	"XML_ERROR_CANT_CHANGE_FEATURE_ONCE_PARSING": 26,
	"XML_ERROR_UNBOUND_PREFIX":                   27,
	"XML_ERROR_UNDECLARING_PREFIX":               28,
	"XML_ERROR_INCOMPLETE_PE":                    29,
	"XML_ERROR_XML_DECL":                         30,
	"XML_ERROR_TEXT_DECL":                        31,
	"XML_ERROR_PUBLICID":                         32,
	"XML_ERROR_SUSPENDED":                        33,
	"XML_ERROR_NOT_SUSPENDED":                    34,
	"XML_ERROR_ABORTED":                          35,
	"XML_ERROR_FINISHED":                         36,
	"XML_ERROR_SUSPEND_PE":                       37,
	"XML_ERROR_RESERVED_PREFIX_XML":              38,
	"XML_ERROR_RESERVED_PREFIX_XMLNS":            39,
	"XML_ERROR_RESERVED_NAMESPACE_URI":           40,
}

// buildExpatErrorsModule constructs pyexpat.errors: each XML_ERROR_* name bound
// to its message string, plus codes (message to code) and messages (code to
// message) maps, matching xml.parsers.expat.errors.
func buildExpatErrorsModule() (objects.Object, error) {
	mod := objects.NewModule("pyexpat.errors", "")
	var codeKeys, codeVals []objects.Object
	var msgKeys, msgVals []objects.Object
	for name, code := range expatErrorConstants {
		msg := expatErrorMessages[code]
		if err := objects.StoreAttr(mod, name, objects.NewStr(msg)); err != nil {
			return nil, err
		}
		codeKeys = append(codeKeys, objects.NewStr(msg))
		codeVals = append(codeVals, objects.NewInt(int64(code)))
		msgKeys = append(msgKeys, objects.NewInt(int64(code)))
		msgVals = append(msgVals, objects.NewStr(msg))
	}
	codes, err := objects.NewDict(codeKeys, codeVals)
	if err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(mod, "codes", codes); err != nil {
		return nil, err
	}
	messages, err := objects.NewDict(msgKeys, msgVals)
	if err != nil {
		return nil, err
	}
	if err := objects.StoreAttr(mod, "messages", messages); err != nil {
		return nil, err
	}
	return mod, nil
}

// buildExpatModelModule constructs pyexpat.model: the XML_CTYPE_* content-model
// and XML_CQUANT_* quantifier constants, matching xml.parsers.expat.model.
func buildExpatModelModule() (objects.Object, error) {
	mod := objects.NewModule("pyexpat.model", "")
	consts := []struct {
		name string
		val  int64
	}{
		{"XML_CTYPE_EMPTY", 1},
		{"XML_CTYPE_ANY", 2},
		{"XML_CTYPE_MIXED", 3},
		{"XML_CTYPE_NAME", 4},
		{"XML_CTYPE_CHOICE", 5},
		{"XML_CTYPE_SEQ", 6},
		{"XML_CQUANT_NONE", 0},
		{"XML_CQUANT_OPT", 1},
		{"XML_CQUANT_REP", 2},
		{"XML_CQUANT_PLUS", 3},
	}
	for _, c := range consts {
		if err := objects.StoreAttr(mod, c.name, objects.NewInt(c.val)); err != nil {
			return nil, err
		}
	}
	return mod, nil
}

// itoa renders a small non-negative int without pulling in strconv, used to
// spell the line and column of an expat syntax error.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
