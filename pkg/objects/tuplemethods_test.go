package objects

import "testing"

// All expected values and messages in this file were probed on
// CPython 3.14.6.

func TestTupleCount(t *testing.T) {
	tup := T(NewInt(1), NewInt(2), NewInt(3), NewInt(2))
	tests := []struct {
		name    string
		args    []Object
		want    string
		wantErr string
	}{
		{"two hits", []Object{NewInt(2)}, "2", ""},
		{"no hit", []Object{NewInt(9)}, "0", ""},
		// 1 == True counts through equals, like CPython.
		{"bool equal", []Object{True}, "1", ""},
		{"no args", nil, "", "TypeError: tuple.count() takes exactly one argument (0 given)"},
		{"two args", []Object{NewInt(1), NewInt(2)}, "",
			"TypeError: tuple.count() takes exactly one argument (2 given)"},
	}
	for _, tt := range tests {
		got, err := CallMethod(tup, "count", tt.args)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		checkRepr(t, tt.name, got, err, tt.want)
	}
}

func TestTupleIndex(t *testing.T) {
	tup := T(NewInt(1), NewInt(2), NewInt(3), NewInt(2))
	tests := []struct {
		name    string
		args    []Object
		want    string
		wantErr string
	}{
		{"basic", []Object{NewInt(2)}, "1", ""},
		{"no args", nil, "", "TypeError: index expected at least 1 argument, got 0"},
		{"four args", []Object{NewInt(1), NewInt(2), NewInt(3), NewInt(4)}, "",
			"TypeError: index expected at most 3 arguments, got 4"},
		{"missing", []Object{NewInt(9)}, "", "ValueError: tuple.index(x): x not in tuple"},
		{"start", []Object{NewInt(2), NewInt(2)}, "3", ""},
		{"start stop", []Object{NewInt(2), NewInt(0), NewInt(2)}, "1", ""},
		{"negative start", []Object{NewInt(2), NewInt(-2)}, "3", ""},
		{"start clamps to 0", []Object{NewInt(1), NewInt(-100)}, "0", ""},
		{"stop clamps to 0", []Object{NewInt(1), NewInt(0), NewInt(-100)}, "",
			"ValueError: tuple.index(x): x not in tuple"},
		{"missing in window", []Object{NewInt(3), NewInt(0), NewInt(1)}, "",
			"ValueError: tuple.index(x): x not in tuple"},
		{"float start", []Object{NewInt(1), NewFloat(1.5)}, "",
			"TypeError: slice indices must be integers or have an __index__ method"},
		{"str start", []Object{NewInt(1), NewStr("a")}, "",
			"TypeError: slice indices must be integers or have an __index__ method"},
	}
	for _, tt := range tests {
		got, err := CallMethod(tup, "index", tt.args)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		checkRepr(t, tt.name, got, err, tt.want)
	}

	// missing str keeps the same wording as missing int
	_, err := CallMethod(T(NewStr("a")), "index", []Object{NewStr("b")})
	checkErr(t, "missing str", err, "ValueError: tuple.index(x): x not in tuple")

	// unknown attribute still falls through to AttributeError
	_, err = CallMethod(tup, "sort", nil)
	checkErr(t, "no such method", err, "AttributeError: 'tuple' object has no attribute 'sort'")
}
