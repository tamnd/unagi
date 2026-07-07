package runtime

import (
	"strings"

	"github.com/tamnd/unagi/pkg/objects"
)

// buildNamedTuple implements collections.namedtuple(typename, field_names, *,
// rename=False, defaults=None, module=None). It parses and validates the names
// the way CPython's factory does, then hands a clean name and field list to
// objects.NewNamedTupleType, which owns the class object and its instances.
// module is accepted and ignored, since unagi does not thread the caller's
// module name through a builtin call.
func buildNamedTuple(a []objects.Object) (objects.Object, error) {
	typename, ok := asGoStr(a[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "Type names and field names must be strings")
	}
	fields, err := parseFieldNames(a[1])
	if err != nil {
		return nil, err
	}
	rename := objects.Truth(a[2])

	if rename {
		seen := map[string]bool{}
		for i, name := range fields {
			if !validIdentifier(name) || pyKeyword(name) || strings.HasPrefix(name, "_") || seen[name] {
				fields[i] = "_" + itoa(i)
			}
			seen[name] = true
		}
	}

	for _, name := range append([]string{typename}, fields...) {
		if !validIdentifier(name) {
			return nil, objects.Raise(objects.ValueError,
				"Type names and field names must be valid identifiers: %s", pyRepr(name))
		}
		if pyKeyword(name) {
			return nil, objects.Raise(objects.ValueError,
				"Type names and field names cannot be a keyword: %s", pyRepr(name))
		}
	}
	seen := map[string]bool{}
	for _, name := range fields {
		if strings.HasPrefix(name, "_") && !rename {
			return nil, objects.Raise(objects.ValueError,
				"Field names cannot start with an underscore: %s", pyRepr(name))
		}
		if seen[name] {
			return nil, objects.Raise(objects.ValueError,
				"Encountered duplicate field name: %s", pyRepr(name))
		}
		seen[name] = true
	}

	var defaults []objects.Object
	if a[3] != objects.None {
		defaults, err = materialize(a[3])
		if err != nil {
			return nil, err
		}
		if len(defaults) > len(fields) {
			return nil, objects.Raise(objects.TypeError, "Got more default values than field names")
		}
	}

	return objects.NewNamedTupleType(typename, fields, defaults)
}

// parseFieldNames turns the field_names argument into a slice of names. A string
// is split on commas and whitespace, matching namedtuple; any other value is
// iterated and each element must be a string.
func parseFieldNames(o objects.Object) ([]string, error) {
	if s, ok := asGoStr(o); ok {
		s = strings.ReplaceAll(s, ",", " ")
		return strings.Fields(s), nil
	}
	elts, err := materialize(o)
	if err != nil {
		return nil, err
	}
	fields := make([]string, len(elts))
	for i, e := range elts {
		name, ok := asGoStr(e)
		if !ok {
			return nil, objects.Raise(objects.TypeError, "Type names and field names must be strings")
		}
		fields[i] = name
	}
	return fields, nil
}

// validIdentifier reports whether s is a valid ASCII Python identifier: a
// leading letter or underscore followed by letters, digits, or underscores.
func validIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			continue
		}
		if i > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

// pyKeyword reports whether s is a Python hard keyword, the set namedtuple
// rejects. The soft keywords (match, case, type, _) are excluded, matching
// keyword.iskeyword.
func pyKeyword(s string) bool {
	switch s {
	case "False", "None", "True", "and", "as", "assert", "async", "await",
		"break", "class", "continue", "def", "del", "elif", "else", "except",
		"finally", "for", "from", "global", "if", "import", "in", "is", "lambda",
		"nonlocal", "not", "or", "pass", "raise", "return", "try", "while",
		"with", "yield":
		return true
	}
	return false
}

// pyRepr spells a string the way Python's repr does, so the validation messages
// quote the offending name exactly as CPython's do.
func pyRepr(s string) string {
	return objects.Repr(objects.NewStr(s))
}

// asGoStr reads a Go string out of a Python str object.
func asGoStr(o objects.Object) (string, bool) {
	if o.TypeName() != "str" {
		return "", false
	}
	return objects.Str(o), true
}

// itoa renders a small non-negative int without pulling in strconv at the call
// site, used only for the rename placeholders _0, _1, and so on.
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
