package runtime

import (
	"strconv"
	"strings"

	"github.com/tamnd/unagi/pkg/objects"
)

// _string is the small C accelerator str.format is built on: string/__init__.py
// opens with `import _string` and drives its Formatter through two parsers,
// formatter_parser and formatter_field_name_split, with no pure-Python fallback.
// The module carries nothing else, so implementing the two parsers is the whole
// capability and unblocks `import string` (Formatter and Template).
//
// The braces in the parsed grammar are all ASCII (`{`, `}`, `!`, `:`, `.`, `[`,
// `]`), none of which can appear inside a multi-byte UTF-8 sequence, so scanning
// the format string byte by byte preserves any multi-byte literal text verbatim.

func init() {
	moduleTable["_string"] = &moduleEntry{builtin: true, exec: initString}
}

func initString(m *objects.Module) error {
	parser := objects.NewFunc("formatter_parser", 1, func(args []objects.Object) (objects.Object, error) {
		s, ok := objects.AsStr(args[0])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "expected str, got %s", args[0].TypeName())
		}
		segs, err := formatterParse(s)
		if err != nil {
			return nil, err
		}
		return objects.NewList(segs), nil
	})
	if err := objects.StoreAttr(m, "formatter_parser", parser); err != nil {
		return err
	}

	split := objects.NewFunc("formatter_field_name_split", 1, func(args []objects.Object) (objects.Object, error) {
		s, ok := objects.AsStr(args[0])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "expected str, got %s", args[0].TypeName())
		}
		first, rest, err := formatterFieldNameSplit(s)
		if err != nil {
			return nil, err
		}
		return objects.NewTuple([]objects.Object{first, objects.NewList(rest)}), nil
	})
	return objects.StoreAttr(m, "formatter_field_name_split", split)
}

// formatterParse implements _string.formatter_parser: it splits a format string
// into (literal_text, field_name, format_spec, conversion) tuples. A doubled
// brace flushes the literal collected so far with a single brace appended and no
// field; a single `{` starts a replacement field whose name, optional `!`
// conversion, and optional `:` format spec run to the matching `}`. A trailing
// literal with no field reads back field, spec, and conversion as None; a field
// with no `:` reads an empty format spec, matching CPython.
func formatterParse(s string) ([]objects.Object, error) {
	var out []objects.Object
	i, n := 0, len(s)
outer:
	for i < n {
		var lit strings.Builder
		for i < n {
			c := s[i]
			if c != '{' && c != '}' {
				lit.WriteByte(c)
				i++
				continue
			}
			// A doubled brace collapses to one literal brace and flushes here.
			if i+1 < n && s[i+1] == c {
				lit.WriteByte(c)
				i += 2
				out = append(out, formatSegment(lit.String(), objects.None, objects.None, objects.None))
				continue outer
			}
			if c == '}' {
				return nil, objects.Raise(objects.ValueError, "Single '}' encountered in format string")
			}
			// A single `{` opens a replacement field; one at the very end has no
			// field body at all, which CPython reports as a stray brace.
			i++
			if i >= n {
				return nil, objects.Raise(objects.ValueError, "Single '{' encountered in format string")
			}
			name, spec, conv, err := formatField(s, &i)
			if err != nil {
				return nil, err
			}
			out = append(out, formatSegment(lit.String(), objects.NewStr(name), spec, conv))
			continue outer
		}
		out = append(out, formatSegment(lit.String(), objects.None, objects.None, objects.None))
	}
	return out, nil
}

// formatSegment builds one (literal_text, field_name, format_spec, conversion)
// tuple.
func formatSegment(lit string, name, spec, conv objects.Object) objects.Object {
	return objects.NewTuple([]objects.Object{objects.NewStr(lit), name, spec, conv})
}

// formatField parses the body of a replacement field, with i pointing just past
// the opening `{`. It returns the field name, the format spec (empty string when
// no `:` is present), and the conversion (None when no `!` is present), leaving i
// just past the closing `}`.
func formatField(s string, i *int) (string, objects.Object, objects.Object, error) {
	n := len(s)
	start := *i
	for *i < n && s[*i] != '!' && s[*i] != ':' && s[*i] != '}' {
		*i++
	}
	if *i >= n {
		return "", nil, nil, objects.Raise(objects.ValueError, "expected '}' before end of string")
	}
	name := s[start:*i]
	conv := objects.Object(objects.None)

	if s[*i] == '!' {
		*i++
		if *i >= n {
			return "", nil, nil, objects.Raise(objects.ValueError, "expected '}' before end of string")
		}
		conv = objects.NewStr(string(s[*i]))
		*i++
		if *i >= n {
			return "", nil, nil, objects.Raise(objects.ValueError, "expected '}' before end of string")
		}
		if s[*i] != ':' && s[*i] != '}' {
			return "", nil, nil, objects.Raise(objects.ValueError, "expected ':' after conversion specifier")
		}
	}

	if s[*i] == ':' {
		*i++
		// The format spec runs to the matching `}` at depth zero; a nested `{`
		// (as in ">{width}") is copied verbatim and balances one `}`.
		specStart := *i
		depth := 0
		for *i < n {
			switch s[*i] {
			case '{':
				depth++
			case '}':
				if depth == 0 {
					spec := objects.NewStr(s[specStart:*i])
					*i++
					return name, spec, conv, nil
				}
				depth--
			}
			*i++
		}
		return "", nil, nil, objects.Raise(objects.ValueError, "expected '}' before end of string")
	}

	// s[*i] == '}': a field with no format spec reads an empty one.
	*i++
	return name, objects.NewStr(""), conv, nil
}

// formatterFieldNameSplit implements _string.formatter_field_name_split: it
// separates a field name into its leading argument (an int when all digits, else
// the name string) and an iterator of (is_attr, key) pairs for the trailing
// `.attr` and `[key]` accessors. A `[key]` whose contents are all digits yields
// an int key. The argument name runs to the first `.` or `[`, so a bare `]`
// stays part of it.
func formatterFieldNameSplit(name string) (objects.Object, []objects.Object, error) {
	n := len(name)
	i := 0
	for i < n && name[i] != '.' && name[i] != '[' {
		i++
	}
	first := indexOrName(name[:i])

	var rest []objects.Object
	for i < n {
		switch name[i] {
		case '.':
			i++
			start := i
			for i < n && name[i] != '.' && name[i] != '[' {
				i++
			}
			rest = append(rest, objects.NewTuple([]objects.Object{objects.NewBool(true), objects.NewStr(name[start:i])}))
		case '[':
			i++
			start := i
			for i < n && name[i] != ']' {
				i++
			}
			if i >= n {
				return nil, nil, objects.Raise(objects.ValueError, "Missing ']' in format string")
			}
			key := indexOrName(name[start:i])
			i++
			rest = append(rest, objects.NewTuple([]objects.Object{objects.NewBool(false), key}))
		default:
			return nil, nil, objects.Raise(objects.ValueError, "Only '.' or '[' may follow ']' in format field specifier")
		}
	}
	return first, rest, nil
}

// indexOrName reads an all-digit run as an int and anything else as the string
// itself, the way _string treats an argument name or a subscript key.
func indexOrName(s string) objects.Object {
	if s != "" && allDigits(s) {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			return objects.NewInt(v)
		}
	}
	return objects.NewStr(s)
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
