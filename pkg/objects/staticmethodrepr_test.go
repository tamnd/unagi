package objects

import (
	"regexp"
	"testing"
)

// TestStaticmethodRepr checks that a staticmethod bound to a type reprs as
// CPython's method-wrapper does, naming its owning type: "<built-in method
// maketrans of type object at 0x...>". Its receiver is the None singleton (the
// staticmethod marker), so the repr resolves the owning type through
// qualnameOwner and BuiltinTypeResolver. Before this case, the None receiver
// fell through to the generic "<function maketrans at ...>" form.
func TestStaticmethodRepr(t *testing.T) {
	strType := NewFunc("str", -1, func([]Object) (Object, error) { return None, nil })
	saved := BuiltinTypeResolver
	BuiltinTypeResolver = func(name string) (Object, bool) {
		if name == "str" {
			return strType, true
		}
		return nil, false
	}
	defer func() { BuiltinTypeResolver = saved }()

	// A staticmethod funcObject: None receiver, owning type recorded.
	sm := &funcObject{name: "maketrans", arity: -1, boundSelf: None, qualnameOwner: "str"}

	addr := regexp.MustCompile(`0x[0-9a-fA-F]+`)
	got := addr.ReplaceAllString(Repr(sm), "0xADDR")
	if got != "<built-in method maketrans of type object at 0xADDR>" {
		t.Fatalf("staticmethod repr = %q", got)
	}
}
