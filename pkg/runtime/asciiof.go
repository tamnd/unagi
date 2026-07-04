package runtime

import (
	"fmt"
	"strings"

	"github.com/tamnd/unagi/pkg/objects"
)

// AsciiOf implements ascii(o): the repr with every non-ASCII character
// escaped. Latin-1 gets \xhh, the rest of the BMP \uhhhh and anything
// above \Uhhhhhhhh. Probed on 3.14: ascii('héllo') is "'h\xe9llo'",
// ascii('日') is "'\\u65e5'" and ascii('😀') is "'\U0001f600'".
func AsciiOf(o objects.Object) (objects.Object, error) {
	r, err := objects.ReprE(o)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	for _, c := range r {
		switch {
		case c < 0x80:
			b.WriteRune(c)
		case c <= 0xff:
			fmt.Fprintf(&b, `\x%02x`, c)
		case c <= 0xffff:
			fmt.Fprintf(&b, `\u%04x`, c)
		default:
			fmt.Fprintf(&b, `\U%08x`, c)
		}
	}
	return objects.NewStr(b.String()), nil
}

// JoinStrs concatenates already-stringified f-string parts into one str
// object. The lowering guarantees every part is a str; anything else
// falls back to its str() text instead of panicking.
func JoinStrs(parts ...objects.Object) objects.Object {
	var b strings.Builder
	for _, p := range parts {
		if s, ok := objects.AsStr(p); ok {
			b.WriteString(s)
		} else {
			b.WriteString(objects.Str(p))
		}
	}
	return objects.NewStr(b.String())
}
