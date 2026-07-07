package runtime

import (
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func TestSysIdentityAttrs(t *testing.T) {
	mo, err := ImportModule("sys")
	if err != nil {
		t.Fatalf("import sys: %v", err)
	}
	attr := func(name string) objects.Object {
		v, err := objects.LoadAttr(mo, name)
		if err != nil {
			t.Fatalf("sys.%s: %v", name, err)
		}
		return v
	}
	if n, _ := objects.AsInt(attr("maxsize")); n != 9223372036854775807 {
		t.Errorf("sys.maxsize = %v, want 9223372036854775807", objects.Str(attr("maxsize")))
	}
	if n, _ := objects.AsInt(attr("maxunicode")); n != 0x10FFFF {
		t.Errorf("sys.maxunicode = %v, want 1114111", objects.Str(attr("maxunicode")))
	}
	if n, _ := objects.AsInt(attr("hexversion")); n != 0x030E06F0 {
		t.Errorf("sys.hexversion = %#x, want 0x030e06f0", n)
	}
	if s, _ := objects.AsStr(attr("byteorder")); s != "little" {
		t.Errorf("sys.byteorder = %q, want little", s)
	}
	// version_info is a 5-tuple whose triple gates the common version check.
	vi := attr("version_info")
	if got := objects.Repr(vi); got != "(3, 14, 6, 'final', 0)" {
		t.Errorf("sys.version_info = %s, want (3, 14, 6, 'final', 0)", got)
	}
}
