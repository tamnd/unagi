package objects

import (
	"math"
	"testing"
)

// Every expected value below is a python3.14 probe under
// PYTHONHASHSEED=0.
func TestPyHash(t *testing.T) {
	frz := func(elts ...Object) Object {
		f, err := NewFrozenset(elts)
		if err != nil {
			t.Fatalf("NewFrozenset: %v", err)
		}
		return f
	}
	tup := func(elts ...Object) Object { return NewTuple(elts) }
	pow2 := func(n uint) Object {
		o := NewInt(1)
		for i := uint(0); i < n; i++ {
			v, err := Mul(o, NewInt(2))
			if err != nil {
				t.Fatal(err)
			}
			o = v
		}
		return o
	}
	neg := func(o Object) Object {
		v, err := Neg(o)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}

	tests := []struct {
		name string
		o    Object
		want int64
	}{
		{"zero", NewInt(0), 0},
		{"one", NewInt(1), 1},
		{"neg one", NewInt(-1), -2},
		{"neg two", NewInt(-2), -2},
		{"modulus", pow2(61), 1},
		{"modulus minus one", neg(pow2(61)), -2},
		{"big", pow2(200), 131072},
		{"big neg", neg(pow2(200)), -131072},
		{"neg 2**60", NewInt(-(1 << 60)), -(1 << 60)},
		{"2**62+5", NewInt((1 << 62) + 5), 7},
		{"float 1.5", NewFloat(1.5), 1152921504606846977},
		{"float -1.5", NewFloat(-1.5), -1152921504606846977},
		{"float 0.5", NewFloat(0.5), 1152921504606846976},
		{"float 0.1", NewFloat(0.1), 230584300921369408},
		{"float small", NewFloat(2.5e-10), 699647011998930896},
		{"float 2**61", NewFloat(math.Pow(2, 61)), 1},
		{"float 1e308", NewFloat(1e308), 156575653125701},
		{"inf", NewFloat(math.Inf(1)), 314159},
		{"neg inf", NewFloat(math.Inf(-1)), -314159},
		{"neg zero", NewFloat(math.Copysign(0, -1)), 0},
		{"none", None, 4238894112},
		{"true", True, 1},
		{"false", False, 0},
		{"str empty", NewStr(""), 0},
		{"str a", NewStr("a"), 4644417185603328019},
		{"str abc", NewStr("abc"), -4594863902769663758},
		{"str 7", NewStr("0123456"), -9148306751049235591},
		{"str 8", NewStr("01234567"), -2720791140458926906},
		{"str 9", NewStr("012345678"), -5215866000161560749},
		{"str ucs1 high", NewStr("héllo"), 6395329678795984700},
		{"str ucs2", NewStr("日本"), 6243316497235261705},
		{"str ucs4", NewStr("😀"), -3536540696076613844},
		{"str long", NewStr("hello world, this is a longer string!"), 2736673927254547960},
		{"tuple empty", tup(), 5740354900026072187},
		{"tuple 1", tup(NewInt(1)), -6644214454873602895},
		{"tuple 1 2", tup(NewInt(1), NewInt(2)), -3550055125485641917},
		{"tuple 1 2 3", tup(NewInt(1), NewInt(2), NewInt(3)), 529344067295497451},
		{"tuple mixed", tup(NewStr("a"), NewFloat(1.5), None), 9046442393729319273},
		{"tuple nested", tup(tup(NewInt(1), NewInt(2)), tup(NewInt(3))), -8303551883679707139},
		{"frozenset empty", frz(), 133146708735736},
		{"frozenset 1", frz(NewInt(1)), -558064481276695278},
		{"frozenset 123", frz(NewInt(1), NewInt(2), NewInt(3)), -272375401224217160},
		{"frozenset mixed", frz(NewStr("a"), NewFloat(2.5)), -5856584121273072565},
		{"frozenset nested", frz(frz(NewInt(1)), NewInt(7)), -4133010611884475318},
		{"range 5", NewRange(0, 5, 1), 5795932985296280846},
		{"range empty", NewRange(0, 0, 1), 2676694398852732306},
		{"range step", NewRange(1, 10, 2), -8580228179051518038},
		{"range empty down", NewRange(7, 3, 1), 2676694398852732306},
	}
	for _, tt := range tests {
		got, err := PyHash(tt.o)
		if err != nil {
			t.Errorf("%s: PyHash error: %v", tt.name, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%s: PyHash = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestPyHashUnhashable(t *testing.T) {
	d, _ := NewDict(nil, nil)
	s, _ := NewSet(nil)
	tests := []struct {
		o    Object
		want string
	}{
		{NewList(nil), "TypeError: unhashable type: 'list'"},
		{d, "TypeError: unhashable type: 'dict'"},
		{s, "TypeError: unhashable type: 'set'"},
		{NewTuple([]Object{NewInt(1), NewList(nil)}), "TypeError: unhashable type: 'list'"},
	}
	for _, tt := range tests {
		_, err := PyHash(tt.o)
		if err == nil || err.Error() != tt.want {
			t.Errorf("PyHash(%s): err = %v, want %s", tt.o.TypeName(), err, tt.want)
		}
	}
}
