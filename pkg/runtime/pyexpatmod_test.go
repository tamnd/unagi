package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

// TestPyexpatModule drives a parser the way plistlib and minidom do: install
// handlers as attributes, feed a document, and check the callback stream. It
// covers the XML declaration, a comment, a processing instruction, attributes,
// text, nesting, and that prolog whitespace is not reported.
func TestPyexpatModule(t *testing.T) {
	mo, err := ImportModule("pyexpat")
	if err != nil {
		t.Fatalf("import pyexpat: %v", err)
	}

	ver, err := objects.LoadAttr(mo, "EXPAT_VERSION")
	if err != nil {
		t.Fatalf("EXPAT_VERSION: %v", err)
	}
	if s, _ := objects.AsStr(ver); s != "expat_2.7.4" {
		t.Fatalf("EXPAT_VERSION = %q", s)
	}

	create, err := objects.LoadAttr(mo, "ParserCreate")
	if err != nil {
		t.Fatalf("ParserCreate: %v", err)
	}
	p, err := objects.Call(create, nil)
	if err != nil {
		t.Fatalf("ParserCreate(): %v", err)
	}

	// An unset handler reads back as None, like the C getset slots.
	h, err := objects.LoadAttr(p, "StartElementHandler")
	if err != nil || h != objects.None {
		t.Fatalf("unset StartElementHandler = %v (%v)", h, err)
	}

	var events []string
	record := func(name string, arity int) {
		fn := objects.NewFunc(name, arity, func(args []objects.Object) (objects.Object, error) {
			parts := name
			for _, a := range args {
				if s, ok := objects.AsStr(a); ok {
					parts += "|" + s
				}
			}
			events = append(events, parts)
			return objects.None, nil
		})
		if err := objects.StoreAttr(p, name, fn); err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
	}
	record("StartElementHandler", 2)
	record("EndElementHandler", 1)
	record("CharacterDataHandler", 1)
	record("CommentHandler", 1)

	parse, err := objects.LoadAttr(p, "Parse")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, err = objects.Call(parse, []objects.Object{
		objects.NewStr("<!-- c --><r a=\"1\">hi<b/>bye</r>"), objects.True,
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := []string{
		"CommentHandler| c ",
		"StartElementHandler|r",
		"CharacterDataHandler|hi",
		"StartElementHandler|b",
		"EndElementHandler|b",
		"CharacterDataHandler|bye",
		"EndElementHandler|r",
	}
	if len(events) != len(want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("event %d = %q, want %q", i, events[i], want[i])
		}
	}
}
