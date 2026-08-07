package objects

import "testing"

// TestCollectionsFromkeysReadIsClassmethod checks reading fromkeys off an
// OrderedDict or a defaultdict resolves the classmethod, so __qualname__ reads
// the short type name (OrderedDict.fromkeys, defaultdict.fromkeys) and __name__
// stays bare, the way CPython inherits the classmethod onto an instance. The
// __self__ = type half needs the runtime's BuiltinTypeResolver and is covered by
// the fixture.
func TestCollectionsFromkeysReadIsClassmethod(t *testing.T) {
	cases := []struct {
		kind     dictKind
		qualname string
	}{
		{orderedDict, "OrderedDict.fromkeys"},
		{defaultDict, "defaultdict.fromkeys"},
	}
	for _, c := range cases {
		d := &dictObject{index: make(map[string]int), kind: c.kind}
		m, err := LoadAttr(d, "fromkeys")
		if err != nil {
			t.Fatalf("%s.fromkeys: %v", d.TypeName(), err)
		}
		if got := loadStr(t, m, "__qualname__"); got != c.qualname {
			t.Fatalf("%s.fromkeys.__qualname__ = %q; want %q", d.TypeName(), got, c.qualname)
		}
		if got := loadStr(t, m, "__name__"); got != "fromkeys" {
			t.Fatalf("%s.fromkeys.__name__ = %q; want %q", d.TypeName(), got, "fromkeys")
		}
	}
}

// TestCollectionsInstanceMethodStillBindsInstance guards the boundary: an
// ordinary method read off an OrderedDict or a defaultdict still binds the
// instance through __self__, so only fromkeys is steered to the classmethod.
func TestCollectionsInstanceMethodStillBindsInstance(t *testing.T) {
	for _, kind := range []dictKind{orderedDict, defaultDict} {
		d := &dictObject{index: make(map[string]int), kind: kind}
		m, err := LoadAttr(d, "get")
		if err != nil {
			t.Fatalf("%s.get: %v", d.TypeName(), err)
		}
		self, err := LoadAttr(m, "__self__")
		if err != nil {
			t.Fatalf("%s.get.__self__: %v", d.TypeName(), err)
		}
		if self != Object(d) {
			t.Fatalf("%s.get.__self__ = %v; want the instance", d.TypeName(), self)
		}
	}
}
