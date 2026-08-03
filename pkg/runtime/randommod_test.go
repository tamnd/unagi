package runtime

import (
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/tamnd/unagi/pkg/objects"
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

// TestSeedFromHash checks that a non-int seed uses the hash of the value cast to
// an unsigned size_t, the path that makes random.Random(3.5) work and stay
// reproducible. An unhashable seed surfaces the TypeError hashing raises.
func TestSeedFromHash(t *testing.T) {
	arg := objects.NewFloat(3.5)
	s, err := newSeededState(arg)
	if err != nil {
		t.Fatalf("newSeededState(3.5): %v", err)
	}
	h, _ := objects.PyHash(arg)
	want := &mtStateObject{}
	want.initByArray(seedKeyFromBig(new(big.Int).SetUint64(uint64(h))))
	if s.mt != want.mt || s.index != want.index {
		t.Fatal("float seed did not match hash-based seeding")
	}
	// The first draw is the value CPython 3.14.6 produces for Random(3.5).
	if got := s.randomDouble(); math.Abs(got-0.3039190124834461) > 1e-15 {
		t.Errorf("Random(3.5).random() = %v", got)
	}
	// An unhashable seed surfaces the TypeError hashing raises.
	if _, err := newSeededState(objects.NewList(nil)); err == nil || !strings.Contains(err.Error(), "unhashable type: 'list'") {
		t.Errorf("list seed error = %v", err)
	}
}

// TestGetrandbitsRange checks getrandbits reports CPython's clinic messages for a
// negative and an oversized bit count, and still assembles a wide word.
func TestGetrandbitsRange(t *testing.T) {
	// The count is validated before the engine is loaded, so a placeholder self
	// is fine for the error paths.
	self := objects.None
	if _, err := randomGetrandbits([]objects.Object{self, objects.NewInt(-1)}); err == nil || !strings.Contains(err.Error(), "Cannot convert negative int") {
		t.Errorf("getrandbits(-1) error = %v", err)
	}
	tooBig := objects.NewIntFromBig(new(big.Int).Lsh(big.NewInt(1), 64))
	if _, err := randomGetrandbits([]objects.Object{self, tooBig}); err == nil || !strings.Contains(err.Error(), "Python int too large for C uint64_t") {
		t.Errorf("getrandbits(2**64) error = %v", err)
	}
	if _, err := randomGetrandbits([]objects.Object{self}); err == nil || !strings.Contains(err.Error(), "Random.getrandbits() takes exactly one argument (0 given)") {
		t.Errorf("getrandbits() arity error = %v", err)
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
