package objects

import (
	"strings"

	"github.com/tamnd/unagi/pkg/sre"
)

// This file holds Pattern.sub and subn, the substitution surface over the _sre
// engine. re compiles a string replacement into a template through the pure
// Python _parser.parse_template and the _sre.template accelerator, which hands
// back the reTemplateObject below: a flat list of literal pieces and group
// indices the substitution walks per match. A callable replacement skips the
// template and runs per match instead.

// reTemplateObject is a compiled replacement template, the result of
// _sre.template. items alternates literal strings (or bytes) with integer group
// indices in the order they appear, so expanding it for a match concatenates the
// literals with the matched group text. _parser.parse_template has already
// checked every index against the pattern's group count.
type reTemplateObject struct {
	items []Object
}

func (*reTemplateObject) TypeName() string { return "re.Template" }

// NewReTemplate builds the compiled template _sre.template returns from the
// literal-and-index list _parser.parse_template produced.
func NewReTemplate(items []Object) Object {
	return &reTemplateObject{items: append([]Object{}, items...)}
}

// CompileReTemplate compiles a string replacement into a template by running the
// re package's own _compile_template, so the template mini-language (\1, \g<n>,
// \g<name>, the octal escapes, and the standard character escapes) parses the
// way CPython spells it. The runtime installs it once the re machinery is
// reachable; it is nil until then, which no substitution can hit because a
// Pattern only exists after re has imported.
var CompileReTemplate func(pattern, repl Object) (Object, error)

// expand builds the replacement text for one match: each literal piece copies
// through and each group index inserts that group's substring, empty when the
// group did not match, matching CPython's sub which fills an unmatched group
// with the empty string rather than raising.
func (t *reTemplateObject) expand(m *matchObject, isbytes bool) (Object, error) {
	pieces := make([]Object, 0, len(t.items))
	for _, it := range t.items {
		if idx, ok := AsInt(it); ok {
			pieces = append(pieces, m.findallGroup(int(idx), isbytes))
			continue
		}
		pieces = append(pieces, it)
	}
	return joinPieces(pieces, isbytes)
}

// joinPieces concatenates str or bytes pieces into one object of the same kind.
func joinPieces(pieces []Object, isbytes bool) (Object, error) {
	if isbytes {
		var out []byte
		for _, p := range pieces {
			b, ok := asBytesLike(p)
			if !ok {
				return nil, Raise(TypeError, "expected a bytes-like object, %s found", p.TypeName())
			}
			out = append(out, b...)
		}
		return NewBytes(out), nil
	}
	var b strings.Builder
	for _, p := range pieces {
		s, ok := AsStr(p)
		if !ok {
			return nil, Raise(TypeError, "expected str instance, %s found", p.TypeName())
		}
		b.WriteString(s)
	}
	return NewStr(b.String()), nil
}

// patternSub implements Pattern.sub and, when wantCount is set, Pattern.subn.
// The arguments are (repl, string, count=0): count caps the substitutions, 0
// meaning every match. A callable repl runs per match, otherwise repl is a
// string template compiled once. The walk copies the text between matches and
// splices the replacement in, stepping one past an empty match so it cannot
// stall, and returns the rebuilt string, or the (string, count) pair for subn.
func patternSub(p *patternObject, args []Object, wantCount bool) (Object, error) {
	name := "sub"
	if wantCount {
		name = "subn"
	}
	if len(args) < 2 {
		return nil, Raise(TypeError, "%s() missing required argument 'string' (pos 2)", name)
	}
	repl := args[0]
	subject := args[1]
	in, isbytes, ok := subjectInput(subject)
	if !ok {
		return nil, Raise(TypeError, "expected string or bytes-like object")
	}
	if isbytes != p.isbytes {
		return nil, Raise(TypeError, "cannot use a %s pattern on a %s-like object",
			kindWord(p.isbytes), kindWord(isbytes))
	}
	count := 0
	if len(args) >= 3 && args[2] != None {
		c, ok := AsIntValue(args[2])
		if !ok {
			return nil, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[2].TypeName())
		}
		count = int(c)
	}

	callable := Callable(repl)
	var tmpl *reTemplateObject
	if !callable {
		if CompileReTemplate == nil {
			return nil, Raise(RuntimeError, "re template compiler unavailable")
		}
		t, err := CompileReTemplate(p, repl)
		if err != nil {
			return nil, err
		}
		tmpl = t.(*reTemplateObject)
	}

	pieces := make([]Object, 0)
	lastEnd := 0
	n := 0
	pos := 0
	endpos := len(in)
	for pos <= endpos {
		if count > 0 && n >= count {
			break
		}
		r, err := sre.Search(in, p.code, p.groups, pos, endpos, false)
		if err != nil {
			return nil, Raise(RuntimeError, "%s", err.Error())
		}
		if !r.Matched {
			break
		}
		m := newMatch(p, subject, in, isbytes, pos, endpos, r).(*matchObject)
		start, end := r.Locs[0], r.Locs[1]
		pieces = append(pieces, sliceInput(in, lastEnd, start, isbytes))
		var rep Object
		if callable {
			rv, err := Call(repl, []Object{m})
			if err != nil {
				return nil, err
			}
			if err := checkReplKind(rv, isbytes); err != nil {
				return nil, err
			}
			rep = rv
		} else {
			rep, err = tmpl.expand(m, isbytes)
			if err != nil {
				return nil, err
			}
		}
		pieces = append(pieces, rep)
		lastEnd = end
		n++
		if end == start {
			pos = end + 1
		} else {
			pos = end
		}
	}
	pieces = append(pieces, sliceInput(in, lastEnd, endpos, isbytes))
	result, err := joinPieces(pieces, isbytes)
	if err != nil {
		return nil, err
	}
	if wantCount {
		return NewTuple([]Object{result, NewInt(int64(n))}), nil
	}
	return result, nil
}

// sliceInput materialises the input window [lo, hi) as bytes for a bytes subject
// and str otherwise, the pieces sub copies between matches.
func sliceInput(in []int32, lo, hi int, isbytes bool) Object {
	if lo < 0 {
		lo = 0
	}
	if hi > len(in) {
		hi = len(in)
	}
	if lo > hi {
		lo = hi
	}
	if isbytes {
		b := make([]byte, hi-lo)
		for i := lo; i < hi; i++ {
			b[i-lo] = byte(in[i])
		}
		return NewBytes(b)
	}
	r := make([]rune, hi-lo)
	for i := lo; i < hi; i++ {
		r[i-lo] = rune(in[i])
	}
	return NewStr(string(r))
}

// checkReplKind rejects a callable replacement whose return kind does not match
// the subject, the str-versus-bytes mismatch CPython raises on.
func checkReplKind(v Object, isbytes bool) error {
	if isbytes {
		if _, ok := asBytesLike(v); !ok {
			return Raise(TypeError, "expected a bytes-like object, %s found", v.TypeName())
		}
		return nil
	}
	if _, ok := AsStr(v); !ok {
		return Raise(TypeError, "expected str instance, %s found", v.TypeName())
	}
	return nil
}
