package objects

import (
	"fmt"
	"strings"
)

// This file holds the compiled-pattern representation the _sre engine is built
// on. re parses and compiles a pattern in pure Python (re._parser and
// re._compiler) into a flat list of ints, the SRE bytecode, then hands that list
// to _sre.compile, which is the boundary this type sits at. The Python layer
// never re-parses the source string here; it passes the finished bytecode and
// the capture-group metadata, and a Pattern stores them for the matcher to run.
// The matcher itself lands in a later slice; this slice is the object and its
// readable surface.

// Pattern compilation flag bits. The values are the SRE_FLAG_* bits re._constants
// defines, the same bits the flags argument to _sre.compile carries.
const (
	SreFlagIgnorecase uint32 = 2
	SreFlagLocale     uint32 = 4
	SreFlagMultiline  uint32 = 8
	SreFlagDotall     uint32 = 16
	SreFlagUnicode    uint32 = 32
	SreFlagVerbose    uint32 = 64
	SreFlagDebug      uint32 = 128
	SreFlagAscii      uint32 = 256
)

// SRE engine limits and identifiers exposed as _sre module attributes. MAGIC
// stamps the bytecode version re._compiler emits, CODESIZE is the byte width of
// one bytecode word in CPython's UCS4 packing, MAXREPEAT is the unbounded-count
// sentinel, and MAXGROUPS caps the group count.
const (
	SreMagic     = 20230612
	SreCodeSize  = 4
	SreMaxRepeat = 0xFFFFFFFF
	SreMaxGroups = 1073741823
)

// patternObject is a compiled regular expression, CPython's re.Pattern. It
// carries the SRE bytecode plus the capture-group metadata the matcher and the
// Match objects read: the source pattern for repr, the flags, the group count,
// the name-to-number index, and the number-to-name tuple.
type patternObject struct {
	pattern    Object // the source str, bytes, or None
	flags      uint32
	code       []uint32
	groups     int
	groupindex Object // dict mapping a named group to its number
	indexgroup Object // tuple mapping a group number to its name
	isbytes    bool
}

func (*patternObject) TypeName() string { return "re.Pattern" }

// NewPattern builds a compiled pattern from the pieces _sre.compile receives: the
// source pattern object, the flag bits, the decoded bytecode, the group count,
// and the two group-name maps. isbytes records whether the pattern was compiled
// from bytes, which the matcher checks against its subject.
func NewPattern(pattern Object, flags uint32, code []uint32, groups int, groupindex, indexgroup Object, isbytes bool) Object {
	return &patternObject{
		pattern:    pattern,
		flags:      flags,
		code:       code,
		groups:     groups,
		groupindex: groupindex,
		indexgroup: indexgroup,
		isbytes:    isbytes,
	}
}

// patternAttr reads the attributes a compiled pattern exposes: the source
// pattern, the flags, the group count, and the name index. CPython also hands
// groupindex back as a read-only mapping; this returns the underlying dict until
// the mapping proxy exists.
func patternAttr(p *patternObject, name string) (Object, error) {
	switch name {
	case "pattern":
		return p.pattern, nil
	case "flags":
		return NewInt(int64(p.flags)), nil
	case "groups":
		return NewInt(int64(p.groups)), nil
	case "groupindex":
		return p.groupindex, nil
	}
	return nil, Raise(AttributeError, "'re.Pattern' object has no attribute '%s'", name)
}

// patternRepr spells re.compile('pattern'), the constructor call that would
// rebuild the pattern, appending the non-default flags when any are set. It
// mirrors CPython's pattern_repr, including the rule that re.UNICODE is implied
// for a string pattern and left off unless a locale or ASCII flag is also set.
func patternRepr(p *patternObject, strict bool) (string, error) {
	pr, err := reprCore(p.pattern, strict)
	if err != nil {
		return "", err
	}
	flags := patternFlagRepr(p.flags, p.isbytes)
	if flags == "" {
		return "re.compile(" + pr + ")", nil
	}
	return "re.compile(" + pr + ", " + flags + ")", nil
}

// patternFlagRepr formats the flag bits the way repr shows them: the named flags
// in a fixed order joined with a bar, then any leftover bits as a hex literal.
// A string pattern carrying only re.UNICODE among the locale and ASCII flags
// drops it, since that is the default for text.
func patternFlagRepr(flags uint32, isbytes bool) string {
	if !isbytes && flags&(SreFlagLocale|SreFlagUnicode|SreFlagAscii) == SreFlagUnicode {
		flags &^= SreFlagUnicode
	}
	named := []struct {
		name string
		bit  uint32
	}{
		{"re.IGNORECASE", SreFlagIgnorecase},
		{"re.LOCALE", SreFlagLocale},
		{"re.MULTILINE", SreFlagMultiline},
		{"re.DOTALL", SreFlagDotall},
		{"re.UNICODE", SreFlagUnicode},
		{"re.VERBOSE", SreFlagVerbose},
		{"re.DEBUG", SreFlagDebug},
		{"re.ASCII", SreFlagAscii},
	}
	var parts []string
	for _, f := range named {
		if flags&f.bit != 0 {
			parts = append(parts, f.name)
			flags &^= f.bit
		}
	}
	if flags != 0 {
		parts = append(parts, fmt.Sprintf("0x%x", flags))
	}
	return strings.Join(parts, "|")
}
