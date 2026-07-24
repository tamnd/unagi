package objects

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/pbkdf2"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha3"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"hash"
)

// _hashlib is the C accelerator (OpenSSL binding) the pure-Python hashlib and
// hmac modules run on. hashlib.py does `from _hashlib import ...` for its named
// constructors, new, pbkdf2_hmac and compare_digest, and hmac.py reads
// _hashlib.hmac_new / hmac_digest / compare_digest and the openssl_* callables.
// The message-digest engines and the HASH/HMAC objects live here in pkg/objects
// next to the other native types; pkg/runtime/hashlibmod.go registers the module
// surface. The digests are backed by the Go standard crypto packages, so the
// output is byte-identical to CPython's OpenSSL. blake2 and scrypt are not in the
// Go standard library and are deferred; hashlib and hmac tolerate their absence.

// hashAlgo describes one message-digest algorithm: its hashlib name, the block
// and digest sizes CPython reports, and a constructor. A regular algorithm holds
// newHash; an extendable-output function (shake) holds newShake and reports a
// digest_size of 0, its digest and hexdigest taking an explicit length.
type hashAlgo struct {
	name       string
	blockSize  int
	digestSize int
	shake      bool
	newHash    func() hash.Hash
	newShake   func() *sha3.SHAKE
}

// hashAlgos is the registry of every algorithm the binding provides, keyed by
// hashlib name. It is the source both for the named constructors and for the
// openssl_md_meth_names set hashlib unions into algorithms_available.
var hashAlgos = map[string]*hashAlgo{
	"md5":       {name: "md5", blockSize: 64, digestSize: 16, newHash: md5.New},
	"sha1":      {name: "sha1", blockSize: 64, digestSize: 20, newHash: sha1.New},
	"sha224":    {name: "sha224", blockSize: 64, digestSize: 28, newHash: sha256.New224},
	"sha256":    {name: "sha256", blockSize: 64, digestSize: 32, newHash: sha256.New},
	"sha384":    {name: "sha384", blockSize: 128, digestSize: 48, newHash: sha512.New384},
	"sha512":    {name: "sha512", blockSize: 128, digestSize: 64, newHash: sha512.New},
	"sha3_224":  {name: "sha3_224", blockSize: 144, digestSize: 28, newHash: func() hash.Hash { return sha3.New224() }},
	"sha3_256":  {name: "sha3_256", blockSize: 136, digestSize: 32, newHash: func() hash.Hash { return sha3.New256() }},
	"sha3_384":  {name: "sha3_384", blockSize: 104, digestSize: 48, newHash: func() hash.Hash { return sha3.New384() }},
	"sha3_512":  {name: "sha3_512", blockSize: 72, digestSize: 64, newHash: func() hash.Hash { return sha3.New512() }},
	"shake_128": {name: "shake_128", blockSize: 168, digestSize: 0, shake: true, newShake: sha3.NewSHAKE128},
	"shake_256": {name: "shake_256", blockSize: 136, digestSize: 0, shake: true, newShake: sha3.NewSHAKE256},
}

// HashlibAlgoNames returns the names of every provided algorithm, backing
// _hashlib.openssl_md_meth_names.
func HashlibAlgoNames() []string {
	names := make([]string, 0, len(hashAlgos))
	for n := range hashAlgos {
		names = append(names, n)
	}
	return names
}

// hashObject is a HASH object: an algorithm plus the message fed so far. The
// message is buffered and the digest is computed on demand from a fresh engine,
// which keeps update cheap and makes copy a plain slice clone with no reliance on
// the engines' binary-marshal state.
type hashObject struct {
	algo *hashAlgo
	data []byte
}

func (h *hashObject) TypeName() string {
	if h.algo.shake {
		return "_hashlib.HASHXOF"
	}
	return "_hashlib.HASH"
}

// NewHashByName builds a HASH object for a named algorithm, optionally seeded
// with data. An unknown name is the ValueError hashlib.new and the openssl_*
// constructors surface.
func NewHashByName(name string, data []byte) (Object, error) {
	algo, ok := hashAlgos[name]
	if !ok {
		return nil, Raise(ValueError, "unsupported hash type %s", name)
	}
	buf := append([]byte(nil), data...)
	return &hashObject{algo: algo, data: buf}, nil
}

// sum computes the current digest of a regular hash from a fresh engine.
func (h *hashObject) sum() []byte {
	e := h.algo.newHash()
	e.Write(h.data)
	return e.Sum(nil)
}

// shakeSum computes length bytes of a shake's extendable output.
func (h *hashObject) shakeSum(length int) []byte {
	s := h.algo.newShake()
	s.Write(h.data)
	out := make([]byte, length)
	s.Read(out)
	return out
}

// hashMethod dispatches the HASH object's methods.
func hashMethod(h *hashObject, name string, args []Object) (Object, error) {
	switch name {
	case "update":
		if len(args) != 1 {
			return nil, Raise(TypeError, "update() takes exactly one argument (%d given)", len(args))
		}
		b, ok := mvBytesLike(args[0])
		if !ok {
			return nil, Raise(TypeError, "object supporting the buffer API required")
		}
		h.data = append(h.data, b...)
		return None, nil
	case "digest":
		if h.algo.shake {
			length, err := shakeLength("digest", args)
			if err != nil {
				return nil, err
			}
			return NewBytes(h.shakeSum(length)), nil
		}
		if len(args) != 0 {
			return nil, Raise(TypeError, "digest() takes no arguments (%d given)", len(args))
		}
		return NewBytes(h.sum()), nil
	case "hexdigest":
		if h.algo.shake {
			length, err := shakeLength("hexdigest", args)
			if err != nil {
				return nil, err
			}
			return NewStr(hex.EncodeToString(h.shakeSum(length))), nil
		}
		if len(args) != 0 {
			return nil, Raise(TypeError, "hexdigest() takes no arguments (%d given)", len(args))
		}
		return NewStr(hex.EncodeToString(h.sum())), nil
	case "copy":
		if len(args) != 0 {
			return nil, Raise(TypeError, "copy() takes no arguments (%d given)", len(args))
		}
		return &hashObject{algo: h.algo, data: append([]byte(nil), h.data...)}, nil
	}
	return nil, noAttr(h, name)
}

// shakeLength reads the required length argument of a shake digest/hexdigest.
func shakeLength(which string, args []Object) (int, error) {
	if len(args) != 1 {
		return 0, Raise(TypeError, "%s() missing required argument 'length' (pos 1)", which)
	}
	n, ok := AsInt(args[0])
	if !ok {
		return 0, Raise(TypeError, "'%s' object cannot be interpreted as an integer", args[0].TypeName())
	}
	if n < 0 {
		return 0, Raise(ValueError, "negative digest length")
	}
	return int(n), nil
}

var hashMethodNames = map[string]bool{
	"update": true, "digest": true, "hexdigest": true, "copy": true,
}

// hashLoadAttr reads the HASH object's data attributes and binds its methods.
func hashLoadAttr(h *hashObject, name string) (Object, error) {
	switch name {
	case "digest_size":
		return NewInt(int64(h.algo.digestSize)), nil
	case "block_size":
		return NewInt(int64(h.algo.blockSize)), nil
	case "name":
		return NewStr(h.algo.name), nil
	}
	if hashMethodNames[name] {
		return builtinMethodValue(h, name), nil
	}
	return nil, noAttr(h, name)
}

// hmacObject is an HMAC object: the inner algorithm, the key, and the message
// fed so far. Like hashObject it buffers the message and computes the MAC on
// demand, so copy is a clone of two slices.
type hmacObject struct {
	algo *hashAlgo
	key  []byte
	data []byte
}

func (h *hmacObject) TypeName() string { return "_hashlib.HMAC" }

// NewHmac builds an HMAC object over a named algorithm, key and optional
// message. A shake or unknown algorithm is the UnsupportedDigestmodError caller
// hmac_new turns the miss into; here it is a plain ValueError the runtime maps.
func NewHmac(name string, key, msg []byte) (Object, error) {
	algo, ok := hashAlgos[name]
	if !ok || algo.shake {
		return nil, Raise(ValueError, "unsupported hash type %s", name)
	}
	return &hmacObject{
		algo: algo,
		key:  append([]byte(nil), key...),
		data: append([]byte(nil), msg...),
	}, nil
}

// mac computes the current HMAC value from a fresh engine.
func (h *hmacObject) mac() []byte {
	m := hmac.New(h.algo.newHash, h.key)
	m.Write(h.data)
	return m.Sum(nil)
}

// hmacObjMethod dispatches the HMAC object's methods.
func hmacObjMethod(h *hmacObject, name string, args []Object) (Object, error) {
	switch name {
	case "update":
		if len(args) != 1 {
			return nil, Raise(TypeError, "update() takes exactly one argument (%d given)", len(args))
		}
		b, ok := mvBytesLike(args[0])
		if !ok {
			return nil, Raise(TypeError, "object supporting the buffer API required")
		}
		h.data = append(h.data, b...)
		return None, nil
	case "digest":
		return NewBytes(h.mac()), nil
	case "hexdigest":
		return NewStr(hex.EncodeToString(h.mac())), nil
	case "copy":
		return &hmacObject{
			algo: h.algo,
			key:  append([]byte(nil), h.key...),
			data: append([]byte(nil), h.data...),
		}, nil
	}
	return nil, noAttr(h, name)
}

var hmacMethodNames = map[string]bool{
	"update": true, "digest": true, "hexdigest": true, "copy": true,
}

// hmacLoadAttr reads the HMAC object's data attributes and binds its methods.
// The name is the hmac-<algo> form CPython's HMAC.name reports.
func hmacLoadAttr(h *hmacObject, name string) (Object, error) {
	switch name {
	case "digest_size":
		return NewInt(int64(h.algo.digestSize)), nil
	case "block_size":
		return NewInt(int64(h.algo.blockSize)), nil
	case "name":
		return NewStr("hmac-" + h.algo.name), nil
	}
	if hmacMethodNames[name] {
		return builtinMethodValue(h, name), nil
	}
	return nil, noAttr(h, name)
}

// HmacDigest computes a one-shot HMAC, backing _hashlib.hmac_digest.
func HmacDigest(name string, key, msg []byte) ([]byte, error) {
	algo, ok := hashAlgos[name]
	if !ok || algo.shake {
		return nil, Raise(ValueError, "unsupported hash type %s", name)
	}
	m := hmac.New(algo.newHash, key)
	m.Write(msg)
	return m.Sum(nil), nil
}

// Pbkdf2Hmac derives a key with PBKDF2-HMAC, backing _hashlib.pbkdf2_hmac. A
// dklen of zero means the underlying digest size, the default hashlib applies
// when dklen is None.
func Pbkdf2Hmac(name string, password, salt []byte, iterations, dklen int) ([]byte, error) {
	algo, ok := hashAlgos[name]
	if !ok || algo.shake {
		return nil, Raise(ValueError, "unsupported hash type %s", name)
	}
	if iterations < 1 {
		return nil, Raise(ValueError, "iteration value must be greater than 0.")
	}
	if dklen == 0 {
		dklen = algo.digestSize
	}
	if dklen < 1 {
		return nil, Raise(ValueError, "key length must be greater than 0.")
	}
	return pbkdf2.Key(algo.newHash, string(password), salt, iterations, dklen)
}

// CompareDigest is the constant-time equality of _hashlib.compare_digest. Two
// strings compare as ASCII (a non-ASCII string is rejected), otherwise both
// operands must be bytes-like; the comparison never short-circuits on content.
func CompareDigest(a, b Object) (Object, error) {
	as, aStr := AsStr(a)
	bs, bStr := AsStr(b)
	if aStr || bStr {
		if !(aStr && bStr) {
			return nil, Raise(TypeError, "unsupported operand types(s) or combination of types")
		}
		if !isASCII(as) || !isASCII(bs) {
			return nil, Raise(TypeError, "comparing strings with non-ASCII characters is not supported")
		}
		return NewBool(constantTimeEqual([]byte(as), []byte(bs))), nil
	}
	ab, aOk := asBytesLike(a)
	bb, bOk := asBytesLike(b)
	if !aOk || !bOk {
		return nil, Raise(TypeError, "unsupported operand types(s) or combination of types")
	}
	return NewBool(constantTimeEqual(ab, bb)), nil
}

// constantTimeEqual reports equality without leaking length or content timing,
// the guarantee compare_digest gives. Unequal lengths are unequal, which
// crypto/subtle already handles by returning zero.
func constantTimeEqual(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}
