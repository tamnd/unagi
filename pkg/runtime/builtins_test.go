package runtime

import (
	"math"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
)

func objs(os ...objects.Object) []objects.Object { return os }

func newList(os ...objects.Object) objects.Object { return objects.NewList(os) }

func i(v int64) objects.Object   { return objects.NewInt(v) }
func f(v float64) objects.Object { return objects.NewFloat(v) }
func s(v string) objects.Object  { return objects.NewStr(v) }

func TestMinMax(t *testing.T) {
	tests := []struct {
		name     string
		fn       func([]objects.Object) (objects.Object, error)
		args     []objects.Object
		want     string
		wantType string
		wantErr  string
	}{
		{"min-2args", Min, objs(i(3), i(2)), "2", "int", ""},
		{"max-2args", Max, objs(i(3), i(2)), "3", "int", ""},
		{"min-iterable", Min, objs(newList(i(4), i(1), i(9))), "1", "int", ""},
		{"max-iterable", Max, objs(newList(i(4), i(9), i(1))), "9", "int", ""},
		{"min-str", Min, objs(s("banana")), "'a'", "str", ""},
		{"min-two-lists", Min, objs(newList(i(2), i(3)), newList(i(1), i(5))), "[1, 5]", "list", ""},
		// First-wins ties, probed: min(1, True) is 1 and min(True, 1) is
		// True; max keeps the first of equals too.
		{"min-tie-int-first", Min, objs(i(1), objects.True), "1", "int", ""},
		{"min-tie-bool-first", Min, objs(objects.True, i(1)), "True", "bool", ""},
		{"max-tie-bool-first", Max, objs(objects.True, f(1)), "True", "bool", ""},
		{"min-tie-iterable", Min, objs(newList(f(1), i(1), objects.True)), "1.0", "float", ""},
		{"max-tie-iterable", Max, objs(newList(i(1), objects.True, f(1))), "1", "int", ""},
		{"min-empty", Min, objs(newList()), "", "", "ValueError: min() iterable argument is empty"},
		{"max-empty", Max, objs(newList()), "", "", "ValueError: max() iterable argument is empty"},
		{"min-0args", Min, nil, "", "", "TypeError: min expected at least 1 argument, got 0"},
		{"max-0args", Max, nil, "", "", "TypeError: max expected at least 1 argument, got 0"},
		{"min-noniter", Min, objs(i(5)), "", "", "TypeError: 'int' object is not iterable"},
		{"min-unorderable", Min, objs(newList(i(1), s("a"))), "", "",
			"TypeError: '<' not supported between instances of 'str' and 'int'"},
		{"max-unorderable", Max, objs(newList(s("a"), i(1))), "", "",
			"TypeError: '>' not supported between instances of 'int' and 'str'"},
		{"min-list-vs-int", Min, objs(newList(i(1), i(2)), i(3)), "", "",
			"TypeError: '<' not supported between instances of 'int' and 'list'"},
	}
	for _, tt := range tests {
		got, err := tt.fn(tt.args)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		if objects.Repr(got) != tt.want || got.TypeName() != tt.wantType {
			t.Errorf("%s: got %s (%s), want %s (%s)",
				tt.name, objects.Repr(got), got.TypeName(), tt.want, tt.wantType)
		}
	}
}

func TestSum(t *testing.T) {
	tests := []struct {
		name    string
		args    []objects.Object
		want    string
		wantErr string
	}{
		{"basic", objs(newList(i(1), i(2), i(3))), "6", ""},
		{"empty", objs(newList()), "0", ""},
		{"start", objs(newList(f(1.5), i(2)), i(10)), "13.5", ""},
		{"start-float", objs(newList(i(1), i(2)), f(0.5)), "3.5", ""},
		{"bools", objs(newList(objects.True, objects.True)), "2", ""},
		{"mixed", objs(newList(i(1), f(2.5))), "3.5", ""},
		{"lists", objs(newList(newList(i(1)), newList(i(2))), newList()), "[1, 2]", ""},
		{"tuples", objs(newList(objects.NewTuple(objs(i(1))), objects.NewTuple(objs(i(2)))), objects.NewTuple(nil)), "(1, 2)", ""},
		{"str-start", objs(newList(s("a"), s("b")), s("")), "",
			"TypeError: sum() can't sum strings [use ''.join(seq) instead]"},
		{"str-start-int-elts", objs(newList(i(1), i(2)), s("x")), "",
			"TypeError: sum() can't sum strings [use ''.join(seq) instead]"},
		{"str-elts-no-start", objs(newList(s("a"), s("b"))), "",
			"TypeError: unsupported operand type(s) for +: 'int' and 'str'"},
		{"lists-no-start", objs(newList(newList(i(1)))), "",
			"TypeError: unsupported operand type(s) for +: 'int' and 'list'"},
		{"noniter", objs(i(5)), "", "TypeError: 'int' object is not iterable"},
		{"0args", nil, "", "TypeError: sum() takes at least 1 positional argument (0 given)"},
		{"3args", objs(newList(), i(1), i(2)), "", "TypeError: sum() takes at most 2 arguments (3 given)"},
	}
	for _, tt := range tests {
		got, err := Sum(tt.args)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		if objects.Repr(got) != tt.want {
			t.Errorf("%s: Sum = %s, want %s", tt.name, objects.Repr(got), tt.want)
		}
	}
}

func TestRound(t *testing.T) {
	// The rows are generated from python3.14 probes: for each (x, nd),
	// the repr and type of round(x[, nd]).
	nd := func(v int64) *int64 { return &v }
	floatRows := []struct {
		x        float64
		nd       *int64
		want     string
		wantType string
	}{
		{2.5, nil, "2", "int"},
		{1.5, nil, "2", "int"},
		{0.5, nil, "0", "int"},
		{-0.5, nil, "0", "int"},
		{-1.5, nil, "-2", "int"},
		{-2.5, nil, "-2", "int"},
		{3.5, nil, "4", "int"},
		{2.4999, nil, "2", "int"},
		{-2.6, nil, "-3", "int"},
		{0.4999999999999999, nil, "0", "int"},
		{math.Copysign(0, -1), nil, "0", "int"},
		{2.675, nd(2), "2.67", "float"},
		{2.665, nd(2), "2.67", "float"},
		{1.005, nd(2), "1.0", "float"},
		{0.125, nd(2), "0.12", "float"},
		{0.375, nd(2), "0.38", "float"},
		{-2.675, nd(2), "-2.67", "float"},
		{2.67, nd(1), "2.7", "float"},
		{1.25, nd(1), "1.2", "float"},
		{1.35, nd(1), "1.4", "float"},
		{-1.25, nd(1), "-1.2", "float"},
		{2.5, nd(0), "2.0", "float"},
		{0.5, nd(0), "0.0", "float"},
		{-0.5, nd(0), "-0.0", "float"},
		{-0.4, nd(0), "-0.0", "float"},
		{math.Copysign(0, -1), nd(2), "-0.0", "float"},
		{123.456, nd(2), "123.46", "float"},
		{123.456, nd(1), "123.5", "float"},
		{123.456, nd(0), "123.0", "float"},
		{123.456, nd(-1), "120.0", "float"},
		{123.456, nd(-2), "100.0", "float"},
		{123.456, nd(-3), "0.0", "float"},
		{350.0, nd(-2), "400.0", "float"},
		{250.0, nd(-2), "200.0", "float"},
		{-350.0, nd(-2), "-400.0", "float"},
		{0.1, nd(1), "0.1", "float"},
		{0.1, nd(5), "0.1", "float"},
		{0.1, nd(17), "0.1", "float"},
		{1e-5, nd(2), "0.0", "float"},
		{5e-324, nd(2), "0.0", "float"},
		{5e-324, nd(324), "5e-324", "float"},
		{1e16, nd(-5), "1e+16", "float"},
		{9007199254740992.0, nd(0), "9007199254740992.0", "float"},
		{1.5, nd(20), "1.5", "float"},
		{1.5, nd(400), "1.5", "float"},
		{1e300, nd(-400), "0.0", "float"},
		{-1e300, nd(-400), "-0.0", "float"},
		{math.Inf(1), nd(2), "inf", "float"},
		{math.NaN(), nd(2), "nan", "float"},
	}
	for _, tt := range floatRows {
		args := objs(f(tt.x))
		label := "round(" + objects.Repr(f(tt.x)) + ")"
		if tt.nd != nil {
			args = append(args, i(*tt.nd))
			label = "round(" + objects.Repr(f(tt.x)) + ", " + objects.Repr(i(*tt.nd)) + ")"
		}
		got, err := Round(args)
		if err != nil {
			t.Errorf("%s: unexpected error %v", label, err)
			continue
		}
		if objects.Repr(got) != tt.want || got.TypeName() != tt.wantType {
			t.Errorf("%s = %s (%s), want %s (%s)",
				label, objects.Repr(got), got.TypeName(), tt.want, tt.wantType)
		}
	}

	intRows := []struct {
		x, nd, want int64
	}{
		{150, -2, 200},
		{250, -2, 200},
		{1234, -2, 1200},
		{-150, -2, -200},
		{25, -1, 20},
		{1234, -5, 0},
		{5, -1, 0},
		{15, -1, 20},
		{-25, -1, -20},
		{999, -3, 1000},
		{500, -3, 0},
		{1500, -3, 2000},
		{0, -2, 0},
		{7, 2, 7},
		{7, 0, 7},
		{150, -25, 0},
	}
	for _, tt := range intRows {
		got, err := Round(objs(i(tt.x), i(tt.nd)))
		if err != nil {
			t.Errorf("round(%d, %d): unexpected error %v", tt.x, tt.nd, err)
			continue
		}
		if objects.Repr(got) != objects.Repr(i(tt.want)) || got.TypeName() != "int" {
			t.Errorf("round(%d, %d) = %s, want %d", tt.x, tt.nd, objects.Repr(got), tt.want)
		}
	}

	if got, err := Round(objs(i(7))); err != nil || objects.Repr(got) != "7" {
		t.Errorf("round(7) = %v, %v", got, err)
	}
	if got, err := Round(objs(objects.True)); err != nil || objects.Repr(got) != "1" || got.TypeName() != "int" {
		t.Errorf("round(True) = %v, %v", got, err)
	}

	errRows := []struct {
		name    string
		args    []objects.Object
		wantErr string
	}{
		{"inf", objs(f(math.Inf(1))), "OverflowError: cannot convert float infinity to integer"},
		{"neg-inf", objs(f(math.Inf(-1))), "OverflowError: cannot convert float infinity to integer"},
		{"nan", objs(f(math.NaN())), "ValueError: cannot convert float NaN to integer"},
		{"str", objs(s("a")), "TypeError: type str doesn't define __round__ method"},
		{"str-nd", objs(f(1.5), s("a")), "TypeError: 'str' object cannot be interpreted as an integer"},
		{"0args", nil, "TypeError: round() missing required argument 'number' (pos 1)"},
		{"3args", objs(i(1), i(2), i(3)), "TypeError: round() takes at most 2 arguments (3 given)"},
		{"max-float-overflow", objs(f(math.MaxFloat64), i(-308)),
			"OverflowError: rounded value too large to represent"},
	}
	for _, tt := range errRows {
		_, err := Round(tt.args)
		checkErr(t, tt.name, err, tt.wantErr)
	}
}

func TestDivMod(t *testing.T) {
	tests := []struct {
		name    string
		a, b    objects.Object
		want    string
		wantErr string
	}{
		{"pos-neg", i(7), i(-2), "(-4, -1)", ""},
		{"neg-pos", i(-7), i(2), "(-4, 1)", ""},
		{"float", f(7.5), i(2), "(3.0, 1.5)", ""},
		{"int-float", i(7), f(2.5), "(2.0, 2.0)", ""},
		{"neg-float", f(-7.5), i(2), "(-4.0, 0.5)", ""},
		{"bools", objects.True, objects.True, "(1, 0)", ""},
		{"int-zero", i(1), i(0), "", "ZeroDivisionError: division by zero"},
		{"float-zero", f(1), f(0), "", "ZeroDivisionError: division by zero"},
		{"int-float-zero", i(1), f(0), "", "ZeroDivisionError: division by zero"},
		{"str", s("a"), i(1), "", "TypeError: unsupported operand type(s) for divmod(): 'str' and 'int'"},
		{"int-none", i(1), objects.None, "", "TypeError: unsupported operand type(s) for divmod(): 'int' and 'NoneType'"},
	}
	for _, tt := range tests {
		got, err := DivMod(tt.a, tt.b)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		if objects.Repr(got) != tt.want {
			t.Errorf("%s: DivMod = %s, want %s", tt.name, objects.Repr(got), tt.want)
		}
	}
}

func TestPow3(t *testing.T) {
	tests := []struct {
		name           string
		base, exp, mod objects.Object
		want           string
		wantErr        string
	}{
		{"inverse", i(3), i(-1), i(7), "5", ""},
		{"basic", i(2), i(10), i(1000), "24", ""},
		{"neg-exp", i(2), i(-3), i(7), "1", ""},
		{"zero-zero", i(0), i(0), i(5), "1", ""},
		{"neg-base", i(-2), i(3), i(7), "6", ""},
		{"neg-mod", i(2), i(3), i(-5), "-2", ""},
		{"neg-mod2", i(3), i(2), i(-7), "-5", ""},
		{"neg-mod3", i(2), i(3), i(-3), "-1", ""},
		{"neg-mod-inverse", i(3), i(-1), i(-7), "-2", ""},
		{"mod-one", i(5), i(3), i(1), "0", ""},
		{"bools", objects.True, objects.True, objects.True, "0", ""},
		{"bool-exp-zero", objects.True, objects.False, i(2), "1", ""},
		{"float-base", f(2), i(3), i(5), "",
			"TypeError: pow() 3rd argument not allowed unless all arguments are integers"},
		{"float-exp", i(2), f(3), i(5), "",
			"TypeError: pow() 3rd argument not allowed unless all arguments are integers"},
		{"float-mod", i(2), i(3), f(5), "",
			"TypeError: pow() 3rd argument not allowed unless all arguments are integers"},
		{"float-and-str", f(2), s("a"), i(3), "",
			"TypeError: pow() 3rd argument not allowed unless all arguments are integers"},
		{"str-base", s("a"), i(2), i(3), "",
			"TypeError: unsupported operand type(s) for ** or pow(): 'str', 'int', 'int'"},
		{"none-base", objects.None, i(2), i(3), "",
			"TypeError: unsupported operand type(s) for ** or pow(): 'NoneType', 'int', 'int'"},
		{"str-mod", i(2), i(3), s("a"), "",
			"TypeError: unsupported operand type(s) for ** or pow(): 'int', 'int', 'str'"},
		{"mod-zero", i(2), i(3), i(0), "", "ValueError: pow() 3rd argument cannot be 0"},
		{"not-invertible", i(2), i(-1), i(4), "", "ValueError: base is not invertible for the given modulus"},
	}
	for _, tt := range tests {
		got, err := Pow3(tt.base, tt.exp, tt.mod)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		if objects.Repr(got) != tt.want {
			t.Errorf("%s: Pow3 = %s, want %s", tt.name, objects.Repr(got), tt.want)
		}
	}
}

func TestBinOctHex(t *testing.T) {
	tests := []struct {
		name    string
		fn      func(objects.Object) (objects.Object, error)
		in      objects.Object
		want    string
		wantErr string
	}{
		{"bin-true", Bin, objects.True, "0b1", ""},
		{"bin-false", Bin, objects.False, "0b0", ""},
		{"bin-neg", Bin, i(-5), "-0b101", ""},
		{"bin-zero", Bin, i(0), "0b0", ""},
		{"oct", Oct, i(8), "0o10", ""},
		{"oct-neg", Oct, i(-8), "-0o10", ""},
		{"hex", Hex, i(255), "0xff", ""},
		{"hex-neg", Hex, i(-255), "-0xff", ""},
		{"hex-zero", Hex, i(0), "0x0", ""},
		{"bin-float", Bin, f(1.5), "", "TypeError: 'float' object cannot be interpreted as an integer"},
		{"hex-str", Hex, s("a"), "", "TypeError: 'str' object cannot be interpreted as an integer"},
		{"oct-none", Oct, objects.None, "", "TypeError: 'NoneType' object cannot be interpreted as an integer"},
	}
	for _, tt := range tests {
		got, err := tt.fn(tt.in)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		if gs, _ := objects.AsStr(got); gs != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, gs, tt.want)
		}
	}
}

func TestOrdChr(t *testing.T) {
	ordTests := []struct {
		name    string
		in      objects.Object
		want    string
		wantErr string
	}{
		{"ascii", s("a"), "97", ""},
		{"bmp", s("€"), "8364", ""},
		{"astral", s("😀"), "128512", ""},
		{"2char", s("ab"), "", "TypeError: ord() expected a character, but string of length 2 found"},
		{"0char", s(""), "", "TypeError: ord() expected a character, but string of length 0 found"},
		{"int", i(1), "", "TypeError: ord() expected string of length 1, but int found"},
	}
	for _, tt := range ordTests {
		got, err := Ord(tt.in)
		if tt.wantErr != "" {
			checkErr(t, "ord "+tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("ord %s: unexpected error %v", tt.name, err)
			continue
		}
		if objects.Repr(got) != tt.want {
			t.Errorf("ord %s: got %s, want %s", tt.name, objects.Repr(got), tt.want)
		}
	}

	chrTests := []struct {
		name    string
		in      objects.Object
		want    string
		wantErr string
	}{
		{"ascii", i(97), "a", ""},
		{"nul", i(0), "\x00", ""},
		{"max", i(0x10FFFF), "\U0010FFFF", ""},
		{"bool", objects.True, "\x01", ""},
		{"neg", i(-1), "", "ValueError: chr() arg not in range(0x110000)"},
		{"too-big", i(0x110000), "", "ValueError: chr() arg not in range(0x110000)"},
		{"float", f(1), "", "TypeError: 'float' object cannot be interpreted as an integer"},
		{"str", s("a"), "", "TypeError: 'str' object cannot be interpreted as an integer"},
		// CPython allows lone surrogates; a Go string cannot, so this is
		// an honest divergence rather than silent corruption.
		{"surrogate", i(0xD800), "",
			"ValueError: chr() arg is a surrogate code point, not representable in this runtime"},
	}
	for _, tt := range chrTests {
		got, err := Chr(tt.in)
		if tt.wantErr != "" {
			checkErr(t, "chr "+tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("chr %s: unexpected error %v", tt.name, err)
			continue
		}
		if gs, _ := objects.AsStr(got); gs != tt.want {
			t.Errorf("chr %s: got %q, want %q", tt.name, gs, tt.want)
		}
	}

	// The ord/chr round trip holds across planes.
	for _, cp := range []int64{0, 65, 0x7FF, 0xFFFD, 0x10000, 0x10FFFF} {
		c, err := Chr(i(cp))
		if err != nil {
			t.Fatalf("chr(%d): %v", cp, err)
		}
		back, err := Ord(c)
		if err != nil {
			t.Fatalf("ord(chr(%d)): %v", cp, err)
		}
		if v, _ := objects.AsInt(back); v != cp {
			t.Errorf("ord(chr(%d)) = %d", cp, v)
		}
	}
}

func TestSorted(t *testing.T) {
	d, err := objects.NewDict(objs(s("b"), s("a")), objs(i(1), i(2)))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		in      objects.Object
		want    string
		wantErr string
	}{
		{"ints", newList(i(3), i(1), i(2)), "[1, 2, 3]", ""},
		{"str", s("cba"), "['a', 'b', 'c']", ""},
		{"dict-keys", d, "['a', 'b']", ""},
		{"mixed-numeric", objects.NewTuple(objs(objects.True, i(0), f(1.5))), "[0, True, 1.5]", ""},
		// Stability, probed: sorted([2.0, 2, 1, True]) keeps the equal
		// elements in input order.
		{"stable", newList(f(2), i(2), i(1), objects.True), "[1, True, 2.0, 2]", ""},
		{"lists", newList(newList(i(2)), newList(i(1), i(3))), "[[1, 3], [2]]", ""},
		{"empty", newList(), "[]", ""},
		{"noniter", i(5), "", "TypeError: 'int' object is not iterable"},
		{"unorderable", newList(i(1), s("a")), "",
			"TypeError: '<' not supported between instances of 'str' and 'int'"},
	}
	for _, tt := range tests {
		got, err := Sorted(tt.in)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		if objects.Repr(got) != tt.want {
			t.Errorf("%s: Sorted = %s, want %s", tt.name, objects.Repr(got), tt.want)
		}
	}

	// Sorting must not reorder the source list.
	src := newList(i(3), i(1), i(2))
	if _, err := Sorted(src); err != nil {
		t.Fatal(err)
	}
	if objects.Repr(src) != "[3, 1, 2]" {
		t.Errorf("Sorted mutated its input: %s", objects.Repr(src))
	}
}

func TestListOfTupleOf(t *testing.T) {
	tests := []struct {
		name    string
		fn      func([]objects.Object) (objects.Object, error)
		args    []objects.Object
		want    string
		wantErr string
	}{
		{"list-empty", ListOf, nil, "[]", ""},
		{"list-str", ListOf, objs(s("ab")), "['a', 'b']", ""},
		{"list-range", ListOf, objs(objects.NewRange(0, 3, 1)), "[0, 1, 2]", ""},
		{"tuple-list", TupleOf, objs(newList(i(1), i(2))), "(1, 2)", ""},
		{"tuple-empty", TupleOf, nil, "()", ""},
		{"list-noniter", ListOf, objs(i(5)), "", "TypeError: 'int' object is not iterable"},
		{"tuple-noniter", TupleOf, objs(i(5)), "", "TypeError: 'int' object is not iterable"},
		{"list-2args", ListOf, objs(i(1), i(2)), "", "TypeError: list expected at most 1 argument, got 2"},
		{"tuple-2args", TupleOf, objs(i(1), i(2)), "", "TypeError: tuple expected at most 1 argument, got 2"},
	}
	for _, tt := range tests {
		got, err := tt.fn(tt.args)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		if objects.Repr(got) != tt.want {
			t.Errorf("%s: got %s, want %s", tt.name, objects.Repr(got), tt.want)
		}
	}

	// list(xs) copies: growing the copy leaves the source alone.
	src := newList(i(1))
	cp, err := ListOf(objs(src))
	if err != nil {
		t.Fatal(err)
	}
	if err := objects.SetItem(cp, i(0), i(9)); err != nil {
		t.Fatal(err)
	}
	if objects.Repr(src) != "[1]" || objects.Repr(cp) != "[9]" {
		t.Errorf("list copy shares storage: src %s, copy %s", objects.Repr(src), objects.Repr(cp))
	}
}

func TestDictOf(t *testing.T) {
	pair := func(k, v objects.Object) objects.Object { return objects.NewTuple(objs(k, v)) }
	src, err := objects.NewDict(objs(s("a")), objs(i(1)))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		args    []objects.Object
		want    string
		wantErr string
	}{
		{"empty", nil, "{}", ""},
		{"pairs", objs(newList(pair(i(1), i(2)), newList(i(3), i(4)))), "{1: 2, 3: 4}", ""},
		{"copy", objs(src), "{'a': 1}", ""},
		{"dup-keys", objs(newList(pair(s("a"), i(1)), pair(s("a"), i(2)))), "{'a': 2}", ""},
		{"str-pairs", objs(newList(s("ab"), s("cd"))), "{'a': 'b', 'c': 'd'}", ""},
		{"elt-nonseq", objs(newList(i(1))), "", "TypeError: object is not iterable"},
		{"elt-nonseq-late", objs(newList(pair(i(1), i(2)), i(3))), "", "TypeError: object is not iterable"},
		{"elt-len3", objs(newList(objects.NewTuple(objs(i(1), i(2), i(3))))), "",
			"ValueError: dictionary update sequence element #0 has length 3; 2 is required"},
		{"elt-len1", objs(newList(objects.NewTuple(objs(i(1))))), "",
			"ValueError: dictionary update sequence element #0 has length 1; 2 is required"},
		{"elt1-len3", objs(newList(pair(i(1), i(2)), objects.NewTuple(objs(i(1), i(2), i(3))))), "",
			"ValueError: dictionary update sequence element #1 has length 3; 2 is required"},
		{"str-arg", objs(s("ab")), "",
			"ValueError: dictionary update sequence element #0 has length 1; 2 is required"},
		{"noniter", objs(i(5)), "", "TypeError: 'int' object is not iterable"},
		{"unhashable-key", objs(newList(pair(newList(i(1)), i(2)))), "",
			"TypeError: cannot use 'list' as a dict key (unhashable type: 'list')"},
		{"2args", objs(i(1), i(2)), "", "TypeError: dict expected at most 1 argument, got 2"},
	}
	for _, tt := range tests {
		got, err := DictOf(tt.args)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		if objects.Repr(got) != tt.want {
			t.Errorf("%s: DictOf = %s, want %s", tt.name, objects.Repr(got), tt.want)
		}
	}

	// dict(d) copies: writing the copy leaves the source alone.
	cp, err := DictOf(objs(src))
	if err != nil {
		t.Fatal(err)
	}
	if err := objects.SetItem(cp, s("b"), i(2)); err != nil {
		t.Fatal(err)
	}
	if objects.Repr(src) != "{'a': 1}" || objects.Repr(cp) != "{'a': 1, 'b': 2}" {
		t.Errorf("dict copy shares storage: src %s, copy %s", objects.Repr(src), objects.Repr(cp))
	}
}

func TestSetOfFrozensetOf(t *testing.T) {
	tests := []struct {
		name    string
		fn      func([]objects.Object) (objects.Object, error)
		args    []objects.Object
		want    string
		wantErr string
	}{
		{"set-empty", SetOf, nil, "set()", ""},
		{"set-dedup", SetOf, objs(newList(i(1), i(2), i(2))), "{1, 2}", ""},
		{"set-str", SetOf, objs(s("aba")), "{'a', 'b'}", ""},
		{"frozenset-empty", FrozensetOf, nil, "frozenset()", ""},
		{"frozenset", FrozensetOf, objs(newList(i(1))), "frozenset({1})", ""},
		{"set-noniter", SetOf, objs(i(5)), "", "TypeError: 'int' object is not iterable"},
		{"set-unhashable", SetOf, objs(newList(newList(i(1)))), "",
			"TypeError: cannot use 'list' as a set element (unhashable type: 'list')"},
		{"set-2args", SetOf, objs(i(1), i(2)), "", "TypeError: set expected at most 1 argument, got 2"},
		{"frozenset-2args", FrozensetOf, objs(i(1), i(2)), "",
			"TypeError: frozenset expected at most 1 argument, got 2"},
	}
	for _, tt := range tests {
		got, err := tt.fn(tt.args)
		if tt.wantErr != "" {
			checkErr(t, tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		if objects.Repr(got) != tt.want {
			t.Errorf("%s: got %s, want %s", tt.name, objects.Repr(got), tt.want)
		}
	}
}

func TestM1Builtins(t *testing.T) {
	for _, name := range []string{
		"min", "max", "sum", "round", "divmod", "bin", "oct", "hex", "ord", "chr",
		"sorted", "reversed", "enumerate", "zip", "list", "tuple", "dict", "set", "frozenset",
	} {
		f, ok := Builtin(name)
		if !ok {
			t.Errorf("Builtin(%q) missing", name)
			continue
		}
		if f.TypeName() != "function" {
			t.Errorf("Builtin(%q) type = %s", name, f.TypeName())
		}
	}
	minF, _ := Builtin("min")
	v, err := objects.Call(minF, objs(i(3), i(1)))
	if err != nil || objects.Repr(v) != "1" {
		t.Errorf("min via Call = %v, %v", v, err)
	}
}

func TestFormatBuiltin(t *testing.T) {
	got, err := Format([]objects.Object{objects.NewFloat(3.14159), objects.NewStr(".2f")})
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := objects.AsStr(got); s != "3.14" {
		t.Fatalf("format(3.14159, .2f) = %q", s)
	}
	got, err = Format([]objects.Object{objects.NewInt(42)})
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := objects.AsStr(got); s != "42" {
		t.Fatalf("format(42) = %q", s)
	}
	if _, err := Format(nil); err == nil || err.Error() != "TypeError: format expected at least 1 argument, got 0" {
		t.Fatalf("format() error = %v", err)
	}
	if _, err := Format([]objects.Object{objects.NewInt(1), objects.NewInt(2)}); err == nil ||
		err.Error() != "TypeError: format() argument 2 must be str, not int" {
		t.Fatalf("format(1, 2) error = %v", err)
	}
	if _, ok := Builtin("format"); !ok {
		t.Fatal("format not registered")
	}
}
