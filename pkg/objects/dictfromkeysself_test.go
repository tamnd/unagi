package objects

import "testing"

// TestPlainDictFromkeysReadIsClassmethod checks reading fromkeys off a plain dict
// resolves the classmethod, so __qualname__ reads dict.fromkeys and __name__ stays
// bare, the way CPython inherits the classmethod onto an instance. The __self__ =
// dict half needs the runtime's BuiltinTypeResolver and is covered by the fixture.
func TestPlainDictFromkeysReadIsClassmethod(t *testing.T) {
	d := &dictObject{index: make(map[string]int)}
	m, err := LoadAttr(d, "fromkeys")
	if err != nil {
		t.Fatalf("{}.fromkeys: %v", err)
	}
	if got := loadStr(t, m, "__qualname__"); got != "dict.fromkeys" {
		t.Fatalf("{}.fromkeys.__qualname__ = %q; want %q", got, "dict.fromkeys")
	}
	if got := loadStr(t, m, "__name__"); got != "fromkeys" {
		t.Fatalf("{}.fromkeys.__name__ = %q; want %q", got, "fromkeys")
	}
}

// TestPlainDictInstanceMethodStillBindsInstance guards the boundary: an ordinary
// dict method read still binds the instance through __self__, so only fromkeys is
// steered to the classmethod.
func TestPlainDictInstanceMethodStillBindsInstance(t *testing.T) {
	d := &dictObject{index: make(map[string]int)}
	m, err := LoadAttr(d, "get")
	if err != nil {
		t.Fatalf("{}.get: %v", err)
	}
	self, err := LoadAttr(m, "__self__")
	if err != nil {
		t.Fatalf("{}.get.__self__: %v", err)
	}
	if self != Object(d) {
		t.Fatalf("{}.get.__self__ = %v; want the dict instance", self)
	}
}
