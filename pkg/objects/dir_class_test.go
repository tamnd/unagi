package objects

import (
	bigInt "math/big"
	"sort"
	"testing"
)

// TestDirNamesOfClass checks dir() of a type object: the object base set plus
// every name across the class's own MRO, so a subclass surfaces the methods it
// inherits and the metaclass-only names stay out. This is what unittest's loader
// walks to collect test methods off a TestCase.
func TestDirNamesOfClass(t *testing.T) {
	noop := func([]Object) (Object, error) { return None, nil }
	base, err := buildClass(nil, "Base", "__main__.Base",
		nil, []string{"helper"}, []Object{NewFunc("helper", -1, noop)}, nil, nil)
	if err != nil {
		t.Fatalf("build Base: %v", err)
	}
	sub, err := buildClass(nil, "Sub", "__main__.Sub",
		[]Object{base}, []string{"foo", "test_a"},
		[]Object{NewFunc("foo", -1, noop), NewFunc("test_a", -1, noop)}, nil, nil)
	if err != nil {
		t.Fatalf("build Sub: %v", err)
	}

	names, ok, err := DirNames(sub)
	if err != nil || !ok {
		t.Fatalf("DirNames(Sub) = _, %v, %v; want ok", ok, err)
	}
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	// Own names, an inherited name, and object base dunders are present.
	for _, want := range []string{"foo", "test_a", "helper", "__init__", "__dict__", "__class__"} {
		if !set[want] {
			t.Errorf("dir(Sub) missing %q", want)
		}
	}
	// Metaclass-only names stay out, matching CPython's dir(cls).
	for _, absent := range []string{"__mro__", "__bases__", "__call__", "__qualname__"} {
		if set[absent] {
			t.Errorf("dir(Sub) unexpectedly has %q", absent)
		}
	}
	// The list is sorted and de-duplicated.
	if !sort.StringsAreSorted(names) {
		t.Errorf("dir(Sub) not sorted: %v", names)
	}
	for i := 1; i < len(names); i++ {
		if names[i] == names[i-1] {
			t.Errorf("dir(Sub) has duplicate %q", names[i])
		}
	}
}

// TestDirNamesOfBuiltinValues checks dir() of a numbers or binary-data builtin
// value returns the exact per-type list, sorted and de-duplicated, and that
// bool reuses int's list while a big int reports the same names as a small one.
func TestDirNamesOfBuiltinValues(t *testing.T) {
	big := NewIntFromBig(new(bigInt.Int).Exp(bigInt.NewInt(10), bigInt.NewInt(80), nil))
	cases := []struct {
		name string
		val  Object
		want []string
	}{
		{"int", NewInt(7), intDirNames},
		{"bigint", big, intDirNames},
		{"bool", True, intDirNames},
		{"float", NewFloat(1.5), floatDirNames},
		{"complex", NewComplex(1, 2), complexDirNames},
		{"bytes", NewBytes([]byte("abc")), bytesDirNames},
		{"bytearray", NewByteArray([]byte("abc")), bytearrayDirNames},
	}
	for _, c := range cases {
		names, ok, err := DirNames(c.val)
		if err != nil || !ok {
			t.Fatalf("DirNames(%s) = _, %v, %v; want ok", c.name, ok, err)
		}
		want := append([]string(nil), c.want...)
		sort.Strings(want)
		if len(names) != len(want) {
			t.Fatalf("dir(%s) has %d names, want %d", c.name, len(names), len(want))
		}
		for i := range want {
			if names[i] != want[i] {
				t.Fatalf("dir(%s)[%d] = %q, want %q", c.name, i, names[i], want[i])
			}
		}
		if !sort.StringsAreSorted(names) {
			t.Errorf("dir(%s) not sorted", c.name)
		}
	}
}

// TestDirNamesOfEmptyClass checks dir() of a class with no body is exactly the
// object base set, the floor CPython reports for a bare class.
func TestDirNamesOfEmptyClass(t *testing.T) {
	empty, err := buildClass(nil, "Empty", "__main__.Empty", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("build Empty: %v", err)
	}
	names, ok, err := DirNames(empty)
	if err != nil || !ok {
		t.Fatalf("DirNames(Empty) = _, %v, %v; want ok", ok, err)
	}
	want := append([]string(nil), objectBaseDirNames...)
	sort.Strings(want)
	if len(names) != len(want) {
		t.Fatalf("dir(Empty) = %v; want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("dir(Empty)[%d] = %q; want %q", i, names[i], want[i])
		}
	}
}
