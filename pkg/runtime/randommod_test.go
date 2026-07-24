package runtime

import (
	"math/big"
	"testing"
)

// seededFromInt is a test helper mirroring the int-seed path.
func seededFromInt(n int64) *mtStateObject {
	s := &mtStateObject{}
	s.initByArray(seedKeyFromBig(big.NewInt(n)))
	return s
}

// TestMTAgainstCPython pins the engine to values captured from CPython 3.14.6
// _random, so a stray constant or off-by-one in the array update is caught
// without booting the interpreter.
func TestMTAgainstCPython(t *testing.T) {
	s := seededFromInt(12345)
	wantFloats := []float64{0.41661987254534116, 0.010169169457068361, 0.8252065092537432}
	for i, want := range wantFloats {
		if got := s.randomDouble(); got != want {
			t.Fatalf("random()[%d] = %v, want %v", i, got, want)
		}
	}

	s = seededFromInt(12345)
	wantBits := []uint32{1789368711, 3146859322, 43676229}
	for i, want := range wantBits {
		if got := s.genrandUint32(); got != want {
			t.Fatalf("getrandbits(32)[%d] = %d, want %d", i, got, want)
		}
	}

	s = seededFromInt(0)
	if got := s.randomDouble(); got != 0.8444218515250481 {
		t.Fatalf("seed(0).random() = %v", got)
	}

	s = &mtStateObject{}
	seed := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 80), big.NewInt(7))
	s.initByArray(seedKeyFromBig(seed))
	if got := s.randomDouble(); got != 0.0074511875167878605 {
		t.Fatalf("seed(2**80+7).random() = %v", got)
	}

	// abs: -42 and 42 seed identically.
	a := seededFromInt(-42)
	b := seededFromInt(42)
	if a.randomDouble() != b.randomDouble() {
		t.Fatal("seed(-42) != seed(42)")
	}

	// getstate head after seed(12345).
	s = seededFromInt(12345)
	if s.mt[0] != 2147483648 || s.mt[1] != 2105189241 || s.mt[2] != 1699489545 || s.index != mtN {
		t.Fatalf("state head = %d %d %d idx %d", s.mt[0], s.mt[1], s.mt[2], s.index)
	}
}

// TestGetrandbits100 checks the wide-word assembly against CPython.
func TestGetrandbits100(t *testing.T) {
	s := seededFromInt(12345)
	result := new(big.Int)
	word := new(big.Int)
	shift := uint(0)
	for remaining := 100; remaining > 0; remaining -= 32 {
		take := uint(32)
		if remaining < 32 {
			take = uint(remaining)
		}
		r := s.genrandUint32() >> (32 - take)
		word.SetUint64(uint64(r))
		word.Lsh(word, shift)
		result.Or(result, word)
		shift += 32
	}
	want, _ := new(big.Int).SetString("1030771796917419777846831192455", 10)
	if result.Cmp(want) != 0 {
		t.Fatalf("getrandbits(100) = %s, want %s", result, want)
	}
}
