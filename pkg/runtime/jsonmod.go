package runtime

import (
	"math"
	"strconv"
	"strings"

	"github.com/tamnd/unagi/pkg/objects"
)

// json is a built-in module here: CPython ships it as pure Python with a C
// accelerator, and the runtime provides the same surface in Go behind the json
// import. This slice is the encoder, json.dumps and json.dump; the decoder,
// json.loads, is the next slice. The encoder follows json.encoder: the scalar
// spellings, the escaping rules that ensure_ascii toggles, the key coercion for
// object keys, and the indent and separator handling for pretty output.

func init() {
	moduleTable["json"] = &moduleEntry{builtin: true, exec: initJSON}
}

func initJSON(m *objects.Module) error {
	dumpsParams := []objects.Param{
		{Name: "obj", Kind: objects.ParamPlain},
		{Name: "skipkeys", Kind: objects.ParamKwOnly},
		{Name: "ensure_ascii", Kind: objects.ParamKwOnly},
		{Name: "allow_nan", Kind: objects.ParamKwOnly},
		{Name: "indent", Kind: objects.ParamKwOnly},
		{Name: "separators", Kind: objects.ParamKwOnly},
		{Name: "default", Kind: objects.ParamKwOnly},
		{Name: "sort_keys", Kind: objects.ParamKwOnly},
	}
	dumpsDefaults := []objects.Object{
		nil,
		objects.False, objects.True, objects.True,
		objects.None, objects.None, objects.None, objects.False,
	}
	dumps := objects.NewFunction("dumps", dumpsParams, dumpsDefaults, func(args []objects.Object) (objects.Object, error) {
		opt, err := jsonOptsFrom(args[1:])
		if err != nil {
			return nil, err
		}
		var b strings.Builder
		if err := jsonEncode(&b, args[0], opt, 0, map[objects.Object]bool{}); err != nil {
			return nil, err
		}
		return objects.NewStr(b.String()), nil
	})
	if err := objects.StoreAttr(m, "dumps", dumps); err != nil {
		return err
	}

	// dump(obj, fp, **kw) writes the same text to a file-like object through its
	// write method, which is what json.dump does.
	dumpParams := append([]objects.Param{
		{Name: "obj", Kind: objects.ParamPlain},
		{Name: "fp", Kind: objects.ParamPlain},
	}, dumpsParams[1:]...)
	dumpDefaults := append([]objects.Object{nil, nil}, dumpsDefaults[1:]...)
	dump := objects.NewFunction("dump", dumpParams, dumpDefaults, func(args []objects.Object) (objects.Object, error) {
		opt, err := jsonOptsFrom(args[2:])
		if err != nil {
			return nil, err
		}
		var b strings.Builder
		if err := jsonEncode(&b, args[0], opt, 0, map[objects.Object]bool{}); err != nil {
			return nil, err
		}
		if _, err := objects.CallMethod(args[1], "write", []objects.Object{objects.NewStr(b.String())}); err != nil {
			return nil, err
		}
		return objects.None, nil
	})
	return objects.StoreAttr(m, "dump", dump)
}

// jsonOpts holds the resolved encoder settings. indent is nil for compact
// output; when set it is the per-level pad string.
type jsonOpts struct {
	skipkeys    bool
	ensureASCII bool
	allowNan    bool
	indent      *string
	itemSep     string
	keySep      string
	defaultFn   objects.Object
	sortKeys    bool
}

// jsonOptsFrom reads the keyword-only tail of dumps or dump, in the order
// skipkeys, ensure_ascii, allow_nan, indent, separators, default, sort_keys.
func jsonOptsFrom(kw []objects.Object) (*jsonOpts, error) {
	opt := &jsonOpts{
		skipkeys:    objects.Truth(kw[0]),
		ensureASCII: objects.Truth(kw[1]),
		allowNan:    objects.Truth(kw[2]),
		sortKeys:    objects.Truth(kw[6]),
		keySep:      ": ",
	}
	if kw[5] != objects.None {
		opt.defaultFn = kw[5]
	}
	if indent := kw[3]; indent != objects.None {
		var pad string
		if n, ok := objects.AsInt(indent); ok {
			if n < 0 {
				n = 0
			}
			pad = strings.Repeat(" ", int(n))
		} else if s, ok := objects.AsStr(indent); ok {
			pad = s
		} else {
			return nil, objects.Raise(objects.TypeError, "indent must be an int or str")
		}
		opt.indent = &pad
		opt.itemSep = ","
	} else {
		opt.itemSep = ", "
	}
	if sep := kw[4]; sep != objects.None {
		item, err := objects.GetItem(sep, objects.NewInt(0))
		if err != nil {
			return nil, err
		}
		key, err := objects.GetItem(sep, objects.NewInt(1))
		if err != nil {
			return nil, err
		}
		is, ok := objects.AsStr(item)
		ks, ok2 := objects.AsStr(key)
		if !ok || !ok2 {
			return nil, objects.Raise(objects.TypeError, "separators must be a pair of strings")
		}
		opt.itemSep, opt.keySep = is, ks
	}
	return opt, nil
}

func jsonNewline(opt *jsonOpts, level int) string {
	if opt.indent == nil {
		return ""
	}
	return "\n" + strings.Repeat(*opt.indent, level)
}

// jsonEncode writes one value. seen guards against circular references in the
// containers currently on the stack.
func jsonEncode(b *strings.Builder, o objects.Object, opt *jsonOpts, level int, seen map[objects.Object]bool) error {
	switch o {
	case objects.None:
		b.WriteString("null")
		return nil
	case objects.True:
		b.WriteString("true")
		return nil
	case objects.False:
		b.WriteString("false")
		return nil
	}
	switch o.TypeName() {
	case "int":
		b.WriteString(objects.Repr(o))
		return nil
	case "float":
		f, _ := objects.AsFloat(o)
		s, err := jsonFloat(f, opt.allowNan)
		if err != nil {
			return err
		}
		b.WriteString(s)
		return nil
	case "str":
		s, _ := objects.AsStr(o)
		b.WriteString(jsonQuote(s, opt.ensureASCII))
		return nil
	case "list", "tuple":
		return jsonEncodeArray(b, o, opt, level, seen)
	case "dict":
		return jsonEncodeObject(b, o, opt, level, seen)
	}
	if opt.defaultFn != nil {
		v, err := objects.Call(opt.defaultFn, []objects.Object{o})
		if err != nil {
			return err
		}
		return jsonEncode(b, v, opt, level, seen)
	}
	return objects.Raise(objects.TypeError, "Object of type %s is not JSON serializable", o.TypeName())
}

func jsonEncodeArray(b *strings.Builder, o objects.Object, opt *jsonOpts, level int, seen map[objects.Object]bool) error {
	if seen[o] {
		return objects.Raise(objects.ValueError, "Circular reference detected")
	}
	seen[o] = true
	defer delete(seen, o)
	n, err := objects.Len(o)
	if err != nil {
		return err
	}
	if n == 0 {
		b.WriteString("[]")
		return nil
	}
	b.WriteByte('[')
	nl := jsonNewline(opt, level+1)
	b.WriteString(nl)
	it, err := objects.Iter(o)
	if err != nil {
		return err
	}
	first := true
	for {
		v, ok, err := it.Next()
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		if !first {
			b.WriteString(opt.itemSep)
			b.WriteString(nl)
		}
		first = false
		if err := jsonEncode(b, v, opt, level+1, seen); err != nil {
			return err
		}
	}
	b.WriteString(jsonNewline(opt, level))
	b.WriteByte(']')
	return nil
}

func jsonEncodeObject(b *strings.Builder, o objects.Object, opt *jsonOpts, level int, seen map[objects.Object]bool) error {
	if seen[o] {
		return objects.Raise(objects.ValueError, "Circular reference detected")
	}
	seen[o] = true
	defer delete(seen, o)

	type pair struct {
		key string
		val objects.Object
	}
	var pairs []pair
	it, err := objects.Iter(o)
	if err != nil {
		return err
	}
	for {
		k, ok, err := it.Next()
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		ks, keep, err := jsonKey(k, opt)
		if err != nil {
			return err
		}
		if !keep {
			continue
		}
		v, err := objects.GetItem(o, k)
		if err != nil {
			return err
		}
		pairs = append(pairs, pair{ks, v})
	}
	if opt.sortKeys {
		// Insertion sort on the encoded string keys: stable and fine for the
		// object sizes json handles.
		for i := 1; i < len(pairs); i++ {
			for j := i; j > 0 && pairs[j-1].key > pairs[j].key; j-- {
				pairs[j-1], pairs[j] = pairs[j], pairs[j-1]
			}
		}
	}
	if len(pairs) == 0 {
		b.WriteString("{}")
		return nil
	}
	b.WriteByte('{')
	nl := jsonNewline(opt, level+1)
	for i, p := range pairs {
		if i > 0 {
			b.WriteString(opt.itemSep)
		}
		b.WriteString(nl)
		b.WriteString(jsonQuote(p.key, opt.ensureASCII))
		b.WriteString(opt.keySep)
		if err := jsonEncode(b, p.val, opt, level+1, seen); err != nil {
			return err
		}
	}
	b.WriteString(jsonNewline(opt, level))
	b.WriteByte('}')
	return nil
}

// jsonKey coerces a dict key to its JSON string form. str stays, the singletons
// and numbers take their JSON spelling, and any other type is a TypeError
// unless skipkeys drops it.
func jsonKey(k objects.Object, opt *jsonOpts) (string, bool, error) {
	switch k {
	case objects.None:
		return "null", true, nil
	case objects.True:
		return "true", true, nil
	case objects.False:
		return "false", true, nil
	}
	switch k.TypeName() {
	case "str":
		s, _ := objects.AsStr(k)
		return s, true, nil
	case "int":
		return objects.Repr(k), true, nil
	case "float":
		f, _ := objects.AsFloat(k)
		s, err := jsonFloat(f, opt.allowNan)
		if err != nil {
			return "", false, err
		}
		return s, true, nil
	}
	if opt.skipkeys {
		return "", false, nil
	}
	return "", false, objects.Raise(objects.TypeError, "keys must be str, int, float, bool or None, not %s", k.TypeName())
}

// jsonFloat spells a float the JSON way: the CPython repr for finite values and
// the JavaScript-flavored words for the non-finite ones, which allow_nan gates.
func jsonFloat(f float64, allowNan bool) (string, error) {
	nonFinite := func(word string) (string, error) {
		if !allowNan {
			return "", objects.Raise(objects.ValueError,
				"Out of range float values are not JSON compliant: %s", pyFloatRepr(f))
		}
		return word, nil
	}
	switch {
	case math.IsNaN(f):
		return nonFinite("NaN")
	case math.IsInf(f, 1):
		return nonFinite("Infinity")
	case math.IsInf(f, -1):
		return nonFinite("-Infinity")
	}
	return objects.Repr(objects.NewFloat(f)), nil
}

// jsonQuote returns the quoted, escaped JSON form of a string. ensureASCII
// escapes every non-ASCII rune as a \u sequence, with a surrogate pair above
// the basic plane; otherwise the rune is kept verbatim.
func jsonQuote(s string, ensureASCII bool) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			switch {
			case r < 0x20:
				b.WriteString(`\u`)
				b.WriteString(jsonHex4(uint16(r)))
			case r < 0x80 || !ensureASCII:
				b.WriteRune(r)
			case r <= 0xFFFF:
				b.WriteString(`\u`)
				b.WriteString(jsonHex4(uint16(r)))
			default:
				r -= 0x10000
				hi := 0xD800 + (r >> 10)
				lo := 0xDC00 + (r & 0x3FF)
				b.WriteString(`\u`)
				b.WriteString(jsonHex4(uint16(hi)))
				b.WriteString(`\u`)
				b.WriteString(jsonHex4(uint16(lo)))
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func jsonHex4(v uint16) string {
	s := strconv.FormatUint(uint64(v), 16)
	return strings.Repeat("0", 4-len(s)) + s
}
