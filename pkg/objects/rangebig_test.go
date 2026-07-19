package objects

import (
	"math/big"
	"testing"
)

// bigPow2 returns a *big.Int equal to 1 << n, past int64 range.
func bigPow2(n uint) *big.Int {
	return new(big.Int).Lsh(big.NewInt(1), n)
}

func TestNewRangeBigStaysFastWhenSmall(t *testing.T) {
	r := NewRangeBig(big.NewInt(0), big.NewInt(3), big.NewInt(1)).(*rangeObject)
	if r.big() {
		t.Fatalf("small bounds should not spill into big fields")
	}
	if r.start != 0 || r.stop != 3 || r.step != 1 {
		t.Fatalf("got start=%d stop=%d step=%d", r.start, r.stop, r.step)
	}
}

func TestBigRangeReprAndLen(t *testing.T) {
	r := NewRangeBig(big.NewInt(0), bigPow2(70), big.NewInt(2))
	got, err := ReprE(r)
	if err != nil {
		t.Fatal(err)
	}
	if want := "range(0, 1180591620717411303424, 2)"; got != want {
		t.Fatalf("repr = %q want %q", got, want)
	}
	if _, err := Len(r); err == nil {
		t.Fatalf("len of a range too large to count should raise OverflowError")
	}
}

func TestBigRangeIterAndIndex(t *testing.T) {
	r := NewRangeBig(big.NewInt(0), bigPow2(200), big.NewInt(1))
	it, err := Iter(r)
	if err != nil {
		t.Fatal(err)
	}
	for want := int64(0); want < 4; want++ {
		v, ok, err := it.Next()
		if err != nil || !ok {
			t.Fatalf("Next %d: ok=%v err=%v", want, ok, err)
		}
		if n, _ := AsInt(v); n != want {
			t.Fatalf("iter[%d] = %v want %d", want, v, want)
		}
	}
	got, err := GetItem(r, NewInt(3))
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := AsInt(got); n != 3 {
		t.Fatalf("r[3] = %v want 3", got)
	}
}

func TestBigRangeContainsMembership(t *testing.T) {
	r := NewRangeBig(big.NewInt(0), bigPow2(70), big.NewInt(2))
	in, err := Contains(r, NewIntFromBig(bigPow2(69)))
	if err != nil {
		t.Fatal(err)
	}
	if in != True {
		t.Fatalf("1<<69 should be in even range")
	}
	odd := new(big.Int).Add(bigPow2(69), big.NewInt(1))
	out, err := Contains(r, NewIntFromBig(odd))
	if err != nil {
		t.Fatal(err)
	}
	if out != False {
		t.Fatalf("(1<<69)+1 is odd, not in the even range")
	}
}

func TestBigRangeEqualsAndHash(t *testing.T) {
	a := NewRangeBig(big.NewInt(0), bigPow2(70), big.NewInt(1))
	b := NewRangeBig(big.NewInt(0), bigPow2(70), big.NewInt(1))
	if !equals(a, b) {
		t.Fatalf("identical big ranges should compare equal")
	}
	c := NewRangeBig(bigPow2(70), bigPow2(71), big.NewInt(1))
	if equals(a, c) {
		t.Fatalf("disjoint big ranges should differ")
	}
	ha, err := PyHash(a)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := PyHash(b)
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Fatalf("equal big ranges must hash equal: %d != %d", ha, hb)
	}
}
