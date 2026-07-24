package runtime

import (
	"crypto/rand"
	"encoding/binary"
	"math/big"

	"github.com/tamnd/unagi/pkg/objects"
)

// _random is the C accelerator (a Mersenne Twister) the pure-Python random
// module runs on. random.py does `class Random(_random.Random)` and leans on the
// base for random(), seed(), getstate(), setstate() and getrandbits(); it layers
// randint/randrange/choice/shuffle/sample on top in Python. So the reproducible
// output hinges entirely on this engine matching CPython's _randommodule.c
// bit-for-bit, which it does: the state array update, the tempering and the
// 53-bit random() formula are the same integer math on every platform.
//
// unagi cannot subclass a native struct type (arrayObject and friends answer
// "bases must be types"), but it can subclass a classObject built with
// objects.NewClass. So Random is such a class, following the posix.DirEntry
// pattern: native methods on a subclassable class, per-instance state in a slot.
// The engine itself is an opaque native Object parked in a hidden _state slot, so
// the 624-word array never lands on the Python heap; getstate/setstate/seed just
// read the slot, type-assert and mutate in place. The slot name is private and
// getstate hands back a proper tuple, so the raw state object is never
// user-visible.

func init() {
	moduleTable["_random"] = &moduleEntry{builtin: true, exec: initRandom}
}

// Mersenne Twister constants from _randommodule.c. All arithmetic is on uint32,
// which wraps mod 2^32 in Go the way the C unsigned math does.
const (
	mtN         = 624
	mtM         = 397
	mtMatrixA   = 0x9908b0df
	mtUpperMask = 0x80000000
	mtLowerMask = 0x7fffffff
)

// rndStateSlot holds the engine. It is a __slots__ entry so a bare
// _random.Random instance keeps no __dict__, matching the C type's layout.
const rndStateSlot = "_state"

// mtStateObject is the opaque engine state stored in the _state slot. It is a
// native Object so it can ride in an instance slot, but it is never surfaced to
// Python: getstate returns a tuple of ints, not this.
type mtStateObject struct {
	mt    [mtN]uint32
	index int
}

func (*mtStateObject) TypeName() string { return "_random.Random.state" }

// initGenrand seeds the array from a single 32-bit word, Knuth's line 25 of
// mt19937ar. index is set past the end so the first draw regenerates.
func (s *mtStateObject) initGenrand(seed uint32) {
	s.mt[0] = seed
	for i := 1; i < mtN; i++ {
		s.mt[i] = 1812433253*(s.mt[i-1]^(s.mt[i-1]>>30)) + uint32(i)
	}
	s.index = mtN
}

// initByArray seeds from a key of 32-bit words, the path both int and urandom
// seeds take. This is init_by_array from mt19937ar verbatim, so a given key
// yields CPython's exact stream.
func (s *mtStateObject) initByArray(key []uint32) {
	s.initGenrand(19650218)
	i, j := 1, 0
	k := mtN
	if len(key) > k {
		k = len(key)
	}
	for ; k > 0; k-- {
		s.mt[i] = (s.mt[i] ^ ((s.mt[i-1] ^ (s.mt[i-1] >> 30)) * 1664525)) + key[j] + uint32(j)
		i++
		j++
		if i >= mtN {
			s.mt[0] = s.mt[mtN-1]
			i = 1
		}
		if j >= len(key) {
			j = 0
		}
	}
	for k = mtN - 1; k > 0; k-- {
		s.mt[i] = (s.mt[i] ^ ((s.mt[i-1] ^ (s.mt[i-1] >> 30)) * 1566083941)) - uint32(i)
		i++
		if i >= mtN {
			s.mt[0] = s.mt[mtN-1]
			i = 1
		}
	}
	s.mt[0] = 0x80000000
	s.index = mtN
}

// genrandUint32 draws the next tempered 32-bit word, regenerating the whole
// array when the cursor runs off the end.
func (s *mtStateObject) genrandUint32() uint32 {
	mag01 := func(y uint32) uint32 {
		if y&1 == 0 {
			return 0
		}
		return mtMatrixA
	}
	if s.index >= mtN {
		var y uint32
		for kk := 0; kk < mtN-mtM; kk++ {
			y = (s.mt[kk] & mtUpperMask) | (s.mt[kk+1] & mtLowerMask)
			s.mt[kk] = s.mt[kk+mtM] ^ (y >> 1) ^ mag01(y)
		}
		for kk := mtN - mtM; kk < mtN-1; kk++ {
			y = (s.mt[kk] & mtUpperMask) | (s.mt[kk+1] & mtLowerMask)
			s.mt[kk] = s.mt[kk+(mtM-mtN)] ^ (y >> 1) ^ mag01(y)
		}
		y = (s.mt[mtN-1] & mtUpperMask) | (s.mt[0] & mtLowerMask)
		s.mt[mtN-1] = s.mt[mtM-1] ^ (y >> 1) ^ mag01(y)
		s.index = 0
	}
	y := s.mt[s.index]
	s.index++
	y ^= y >> 11
	y ^= (y << 7) & 0x9d2c5680
	y ^= (y << 15) & 0xefc60000
	y ^= y >> 18
	return y
}

// randomDouble is genrand_res53: two draws give a 53-bit mantissa uniformly in
// [0, 1). The literals are the exact ones in _randommodule.c so the rounding
// matches to the last bit.
func (s *mtStateObject) randomDouble() float64 {
	a := s.genrandUint32() >> 5
	b := s.genrandUint32() >> 6
	return (float64(a)*67108864.0 + float64(b)) * (1.0 / 9007199254740992.0)
}

// seedFromUint32Words builds the key CPython builds from an int seed: the
// little-endian 32-bit words of abs(n), with a lone zero word for n == 0.
func seedKeyFromBig(n *big.Int) []uint32 {
	n = new(big.Int).Abs(n)
	if n.Sign() == 0 {
		return []uint32{0}
	}
	mask := big.NewInt(0xffffffff)
	tmp := new(big.Int).Set(n)
	word := new(big.Int)
	var key []uint32
	for tmp.Sign() > 0 {
		word.And(tmp, mask)
		key = append(key, uint32(word.Uint64()))
		tmp.Rsh(tmp, 32)
	}
	return key
}

// seedFromUrandom seeds from 32 os-random bytes, matching CPython's default of
// drawing entropy when no seed is given. The exact bytes do not matter since
// unseeded output is never asserted; it just has to be nondeterministic.
func (s *mtStateObject) seedFromUrandom() error {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return objects.Raise(objects.RuntimeError, "_random: cannot read os entropy: %s", err.Error())
	}
	key := make([]uint32, 8)
	for i := range key {
		key[i] = binary.LittleEndian.Uint32(buf[i*4 : i*4+4])
	}
	s.initByArray(key)
	return nil
}

// newSeededState builds an engine seeded from an argument the way
// _random.Random.seed does: None draws from urandom, an int uses all its bits.
// random.py hashes str/bytes to an int itself, so only int and None reach here.
func newSeededState(arg objects.Object) (*mtStateObject, error) {
	s := &mtStateObject{}
	if arg == nil || arg == objects.None {
		if err := s.seedFromUrandom(); err != nil {
			return nil, err
		}
		return s, nil
	}
	b, ok := objects.AsBigInt(arg)
	if !ok {
		return nil, objects.Raise(objects.TypeError,
			"The only supported seed types are: None,\nint, float, str, bytes, and bytearray.")
	}
	s.initByArray(seedKeyFromBig(b))
	return s, nil
}

// loadState reads the engine out of the instance's hidden slot.
func loadState(self objects.Object) (*mtStateObject, error) {
	v, err := objects.LoadAttr(self, rndStateSlot)
	if err != nil {
		return nil, err
	}
	s, ok := v.(*mtStateObject)
	if !ok {
		return nil, objects.Raise(objects.RuntimeError, "_random.Random: engine state is missing")
	}
	return s, nil
}

func buildRandomClass() (objects.Object, error) {
	slots := objects.NewTuple([]objects.Object{objects.NewStr(rndStateSlot)})
	names := []string{
		"__slots__", "__init__",
		"random", "seed", "getrandbits", "getstate", "setstate",
	}
	vals := []objects.Object{
		slots,
		objects.NewMethod("__init__", -1, randomInit),
		objects.NewMethod("random", 1, randomRandom),
		objects.NewMethod("seed", -1, randomSeed),
		objects.NewMethod("getrandbits", 2, randomGetrandbits),
		objects.NewMethod("getstate", 1, randomGetstate),
		objects.NewMethod("setstate", 2, randomSetstate),
	}
	return objects.NewClass("Random", "_random.Random", nil, names, vals, nil, nil)
}

func initRandom(m *objects.Module) error {
	cls, err := buildRandomClass()
	if err != nil {
		return err
	}
	return objects.StoreAttr(m, "Random", cls)
}

// randomInit seeds a fresh instance. _random.Random.__init__ takes an optional
// seed; random.py's subclass overrides __init__ and never chains here, so this
// runs only for a direct _random.Random(), where a usable engine has to exist
// before the first draw. A bare call seeds from urandom.
func randomInit(args []objects.Object) (objects.Object, error) {
	if len(args) > 2 {
		return nil, objects.Raise(objects.TypeError,
			"Random() takes at most 1 argument (%d given)", len(args)-1)
	}
	var arg objects.Object
	if len(args) == 2 {
		arg = args[1]
	}
	s, err := newSeededState(arg)
	if err != nil {
		return nil, err
	}
	return objects.None, objects.StoreAttr(args[0], rndStateSlot, s)
}

// randomSeed reseeds the engine in place. random.py's Random.seed calls
// super().seed(a) with a single int-or-None argument.
func randomSeed(args []objects.Object) (objects.Object, error) {
	if len(args) > 2 {
		return nil, objects.Raise(objects.TypeError,
			"seed() takes at most 1 argument (%d given)", len(args)-1)
	}
	var arg objects.Object
	if len(args) == 2 {
		arg = args[1]
	}
	s, err := newSeededState(arg)
	if err != nil {
		return nil, err
	}
	return objects.None, objects.StoreAttr(args[0], rndStateSlot, s)
}

func randomRandom(args []objects.Object) (objects.Object, error) {
	s, err := loadState(args[0])
	if err != nil {
		return nil, err
	}
	return objects.NewFloat(s.randomDouble()), nil
}

// randomGetrandbits returns a k-bit random int, assembled low words first so a
// bignum comes out with the same bit layout CPython produces. k == 0 yields 0
// and a negative k is a ValueError, both matching 3.14.
func randomGetrandbits(args []objects.Object) (objects.Object, error) {
	k, ok := objects.AsInt(args[1])
	if !ok {
		if objects.IsBigInt(args[1]) {
			return nil, objects.Raise(objects.OverflowError, "number of bits is too large")
		}
		return nil, objects.Raise(objects.TypeError, "'%s' object cannot be interpreted as an integer", args[1].TypeName())
	}
	if k < 0 {
		return nil, objects.Raise(objects.ValueError, "number of bits must be non-negative")
	}
	s, err := loadState(args[0])
	if err != nil {
		return nil, err
	}
	if k == 0 {
		return objects.NewInt(0), nil
	}
	if k <= 32 {
		return objects.NewInt(int64(s.genrandUint32() >> (32 - uint(k)))), nil
	}
	result := new(big.Int)
	word := new(big.Int)
	shift := uint(0)
	for remaining := k; remaining > 0; remaining -= 32 {
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
	return objects.NewIntFromBig(result), nil
}

// randomGetstate returns the 624 state words plus the cursor as 625 ints, the
// tuple random.py stows and later feeds back to setstate.
func randomGetstate(args []objects.Object) (objects.Object, error) {
	s, err := loadState(args[0])
	if err != nil {
		return nil, err
	}
	elts := make([]objects.Object, mtN+1)
	for i := 0; i < mtN; i++ {
		elts[i] = objects.NewInt(int64(s.mt[i]))
	}
	elts[mtN] = objects.NewInt(int64(s.index))
	return objects.NewTuple(elts), nil
}

// randomSetstate restores the engine from a 625-int sequence produced by
// getstate. The words are masked to uint32 and the cursor must be 0..624, the
// range CPython validates.
func randomSetstate(args []objects.Object) (objects.Object, error) {
	s, err := loadState(args[0])
	if err != nil {
		return nil, err
	}
	state := args[1]
	n, err := objects.Len(state)
	if err != nil {
		return nil, err
	}
	if n != mtN+1 {
		return nil, objects.Raise(objects.ValueError, "state vector is the wrong size")
	}
	var next mtStateObject
	for i := 0; i < mtN; i++ {
		item, err := objects.GetItem(state, objects.NewInt(int64(i)))
		if err != nil {
			return nil, err
		}
		w, ok := stateWord(item)
		if !ok {
			return nil, objects.Raise(objects.ValueError, "state vector must contain integers")
		}
		next.mt[i] = w
	}
	idxObj, err := objects.GetItem(state, objects.NewInt(int64(mtN)))
	if err != nil {
		return nil, err
	}
	idx, ok := objects.AsInt(idxObj)
	if !ok {
		return nil, objects.Raise(objects.ValueError, "state vector must contain integers")
	}
	if idx < 0 || idx > mtN {
		return nil, objects.Raise(objects.ValueError, "invalid state")
	}
	next.index = int(idx)
	*s = next
	return objects.None, nil
}

// stateWord reads one setstate element as a uint32, masking to 32 bits the way
// the C init_by_array-style loader keeps only the low bits. A bignum word (a
// full 2^32-1 value round-trips as a small int here, so bignum is unexpected)
// still masks cleanly.
func stateWord(o objects.Object) (uint32, bool) {
	if v, ok := objects.AsInt(o); ok {
		return uint32(v), true
	}
	if b, ok := objects.AsBigInt(o); ok {
		return uint32(new(big.Int).And(b, big.NewInt(0xffffffff)).Uint64()), true
	}
	return 0, false
}
