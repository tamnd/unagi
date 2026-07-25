package objects

import "testing"

// A defaultdict is a dict subclass in CPython, so a defaultdict value answers
// isinstance against both its own module-qualified type and dict.
func TestDefaultDictInstanceOfDict(t *testing.T) {
	dd, err := NewDefaultDict(None, nil, nil)
	if err != nil {
		t.Fatalf("NewDefaultDict: %v", err)
	}
	if !instanceOfBuiltin(dd, "collections.defaultdict") {
		t.Error("defaultdict is not an instance of collections.defaultdict")
	}
	if !instanceOfBuiltin(dd, "dict") {
		t.Error("defaultdict is not an instance of dict")
	}
}

// Counter and OrderedDict are dict subclasses too, so they also answer to dict.
func TestDictKindsInstanceOfDict(t *testing.T) {
	for _, name := range []string{"Counter", "OrderedDict"} {
		d, err := NewDict(nil, nil)
		if err != nil {
			t.Fatalf("NewDict: %v", err)
		}
		switch name {
		case "Counter":
			d.(*dictObject).kind = counterDict
		case "OrderedDict":
			d.(*dictObject).kind = orderedDict
		}
		if !instanceOfBuiltin(d, "dict") {
			t.Errorf("%s is not an instance of dict", name)
		}
	}
}

// builtinBaseMatches bridges a defaultdict subclass's recorded base layout to
// the type names it must answer to: its own qualified type and dict, but no
// unrelated builtin.
func TestBuiltinBaseMatches(t *testing.T) {
	cases := []struct {
		base, name string
		want       bool
	}{
		{"defaultdict", "defaultdict", true},
		{"defaultdict", "collections.defaultdict", true},
		{"defaultdict", "dict", true},
		{"defaultdict", "list", false},
		{"dict", "dict", true},
		{"dict", "defaultdict", false},
		{"list", "list", true},
	}
	for _, c := range cases {
		if got := builtinBaseMatches(c.base, c.name); got != c.want {
			t.Errorf("builtinBaseMatches(%q, %q) = %v, want %v", c.base, c.name, got, c.want)
		}
	}
}
