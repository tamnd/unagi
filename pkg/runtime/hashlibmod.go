package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _hashlib is the C accelerator (an OpenSSL binding in CPython) the pure-Python
// hashlib and hmac modules run on. hashlib.py does `import _hashlib` and, when it
// succeeds, routes new, the named constructors, pbkdf2_hmac and compare_digest
// through it; hmac.py reads _hashlib.hmac_new / hmac_digest / compare_digest and
// the openssl_* callables. This file registers the module and its callables; the
// digest engines and the HASH/HMAC objects live in pkg/objects.
//
// The digests come from the Go standard crypto packages, so the output is
// byte-identical to CPython's OpenSSL. blake2 and scrypt are not in the Go
// standard library and are deferred. blake2b/blake2s still have to be importable,
// because hashlib.py iterates its guaranteed algorithms at import time and a
// missing one would surface a logging error rather than the quiet skip the pure
// fallback intends, so a minimal _blake2 stub keeps `import hashlib` clean while
// the algorithm itself stays unimplemented.

// hashlibUnsupportedDigestmod is _hashlib.UnsupportedDigestmodError, the
// ValueError subclass hmac.py catches when a digestmod is not one this binding
// provides. It is built in initHashlib and captured for hmac_new to raise.
var hashlibUnsupportedDigestmod objects.Object

// hashlibOpensslAlgo maps each openssl_<name> constructor object back to its
// algorithm name, so hmac_new and hmac_digest can accept a constructor as their
// digestmod the way hmac.py passes hashlib.sha256.
var hashlibOpensslAlgo = map[objects.Object]string{}

func init() {
	moduleTable["_hashlib"] = &moduleEntry{builtin: true, exec: initHashlib}
	moduleTable["_blake2"] = &moduleEntry{builtin: true, exec: initBlake2Stub}
}

// initBlake2Stub provides an importable _blake2 with blake2b and blake2s so
// hashlib's guaranteed-algorithm loop finds them, without implementing the
// algorithm: calling either constructor raises, since blake2 is deferred.
func initBlake2Stub(m *objects.Module) error {
	for _, name := range []string{"blake2b", "blake2s"} {
		n := name
		fn := objects.NewFuncKw(n, func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
			return nil, objects.Raise(objects.ValueError, "unsupported hash type %s", n)
		})
		if err := objects.StoreAttr(m, n, fn); err != nil {
			return err
		}
	}
	return nil
}

func initHashlib(m *objects.Module) error {
	valueError, ok := objects.ExcClassValue("ValueError")
	if !ok {
		return objects.Raise(objects.RuntimeError, "_hashlib: ValueError base is unavailable")
	}
	excCls, err := objects.NewClass("UnsupportedDigestmodError", "_hashlib.UnsupportedDigestmodError",
		[]objects.Object{valueError}, nil, nil, nil, nil)
	if err != nil {
		return err
	}
	hashlibUnsupportedDigestmod = excCls
	if err := objects.StoreAttr(m, "UnsupportedDigestmodError", excCls); err != nil {
		return err
	}

	// The named constructors, openssl_<name>(data=b'', *, usedforsecurity=True).
	// hashlib binds each as its module-level constructor and hmac.py passes them
	// as a digestmod, so every one is the same callable type.
	for _, name := range objects.HashlibAlgoNames() {
		n := name
		fn := objects.NewFuncKw("openssl_"+n, func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
			data, err := hashlibConstructorArgs("openssl_"+n, pos, kwNames)
			if err != nil {
				return nil, err
			}
			return objects.NewHashByName(n, data)
		})
		hashlibOpensslAlgo[fn] = n
		if err := objects.StoreAttr(m, "openssl_"+n, fn); err != nil {
			return err
		}
	}

	// openssl_md_meth_names: the frozenset of available algorithm names hashlib
	// unions into algorithms_available.
	nameObjs := make([]objects.Object, 0)
	for _, n := range objects.HashlibAlgoNames() {
		nameObjs = append(nameObjs, objects.NewStr(n))
	}
	methNames, err := objects.NewFrozenset(nameObjs)
	if err != nil {
		return err
	}
	if err := objects.StoreAttr(m, "openssl_md_meth_names", methNames); err != nil {
		return err
	}

	entries := []struct {
		name string
		val  objects.Object
	}{
		{"new", objects.NewFuncKw("new", hashlibNew)},
		{"compare_digest", objects.NewFunc("compare_digest", 2, hashlibCompareDigest)},
		{"hmac_new", objects.NewFuncKw("hmac_new", hashlibHmacNew)},
		{"hmac_digest", objects.NewFunc("hmac_digest", 3, hashlibHmacDigest)},
		{"pbkdf2_hmac", objects.NewFuncKw("pbkdf2_hmac", hashlibPbkdf2)},
	}
	for _, e := range entries {
		if err := objects.StoreAttr(m, e.name, e.val); err != nil {
			return err
		}
	}
	return nil
}

// hashlibConstructorArgs reads the optional data argument and the keyword-only
// usedforsecurity flag shared by new and the openssl_<name> constructors.
// usedforsecurity is accepted and ignored: the digests are always computed.
func hashlibConstructorArgs(fname string, pos []objects.Object, kwNames []string) ([]byte, error) {
	if len(pos) > 1 {
		return nil, objects.Raise(objects.TypeError, "%s() takes at most 1 positional argument (%d given)", fname, len(pos))
	}
	for _, k := range kwNames {
		if k != "usedforsecurity" && k != "string" && k != "data" {
			return nil, objects.Raise(objects.TypeError, "%s() got an unexpected keyword argument '%s'", fname, k)
		}
	}
	if len(pos) == 1 {
		b, ok := objects.AsBufferBytes(pos[0])
		if !ok {
			return nil, objects.Raise(objects.TypeError, "object supporting the buffer API required")
		}
		return b, nil
	}
	return nil, nil
}

// hashlibNew is _hashlib.new(name, data=b”, *, usedforsecurity=True).
func hashlibNew(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 1 {
		return nil, objects.Raise(objects.TypeError, "new() missing required argument 'name' (pos 1)")
	}
	name, ok := objects.AsStr(pos[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "name must be a string")
	}
	data, err := hashlibConstructorArgs("new", pos[1:], kwNames)
	if err != nil {
		return nil, err
	}
	return objects.NewHashByName(name, data)
}

// hashlibCompareDigest is _hashlib.compare_digest(a, b).
func hashlibCompareDigest(args []objects.Object) (objects.Object, error) {
	return objects.CompareDigest(args[0], args[1])
}

// hashlibResolveDigestmod maps a digestmod, either a name string or an
// openssl_<name> constructor, to its algorithm name. A miss is the
// UnsupportedDigestmodError hmac.py catches.
func hashlibResolveDigestmod(digestmod objects.Object) (string, error) {
	if digestmod == nil {
		return "", hashlibUnsupportedError("Missing required argument 'digestmod'.")
	}
	if s, ok := objects.AsStr(digestmod); ok {
		return s, nil
	}
	if n, ok := hashlibOpensslAlgo[digestmod]; ok {
		return n, nil
	}
	return "", hashlibUnsupportedError("Unsupported digestmod")
}

// hashlibUnsupportedError raises an UnsupportedDigestmodError carrying msg.
func hashlibUnsupportedError(msg string) error {
	if hashlibUnsupportedDigestmod != nil {
		if inst, err := objects.Call(hashlibUnsupportedDigestmod, []objects.Object{objects.NewStr(msg)}); err == nil {
			if e, ok := inst.(error); ok {
				return e
			}
		}
	}
	return objects.Raise(objects.ValueError, "%s", msg)
}

// hashlibBytesArg reads a required bytes-like argument, the way the binding
// takes a key or message buffer.
func hashlibBytesArg(o objects.Object) ([]byte, error) {
	if o == objects.None {
		return nil, nil
	}
	b, ok := objects.AsBufferBytes(o)
	if !ok {
		return nil, objects.Raise(objects.TypeError, "object supporting the buffer API required")
	}
	return b, nil
}

// hashlibHmacNew is _hashlib.hmac_new(key, msg=b”, digestmod=...). hmac.py calls
// it with the key and message positional and digestmod as a keyword.
func hashlibHmacNew(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 1 || len(pos) > 2 {
		return nil, objects.Raise(objects.TypeError, "hmac_new() takes 1 or 2 positional arguments (%d given)", len(pos))
	}
	key, err := hashlibBytesArg(pos[0])
	if err != nil {
		return nil, err
	}
	var msg []byte
	if len(pos) == 2 {
		if msg, err = hashlibBytesArg(pos[1]); err != nil {
			return nil, err
		}
	}
	var digestmod objects.Object
	for i, k := range kwNames {
		switch k {
		case "digestmod":
			digestmod = kwVals[i]
		case "msg":
			if msg, err = hashlibBytesArg(kwVals[i]); err != nil {
				return nil, err
			}
		default:
			return nil, objects.Raise(objects.TypeError, "hmac_new() got an unexpected keyword argument '%s'", k)
		}
	}
	name, err := hashlibResolveDigestmod(digestmod)
	if err != nil {
		return nil, err
	}
	obj, err := objects.NewHmac(name, key, msg)
	if err != nil {
		return nil, hashlibUnsupportedError("unsupported hash type " + name)
	}
	return obj, nil
}

// hashlibHmacDigest is _hashlib.hmac_digest(key, msg, digest).
func hashlibHmacDigest(args []objects.Object) (objects.Object, error) {
	key, err := hashlibBytesArg(args[0])
	if err != nil {
		return nil, err
	}
	msg, err := hashlibBytesArg(args[1])
	if err != nil {
		return nil, err
	}
	name, err := hashlibResolveDigestmod(args[2])
	if err != nil {
		return nil, err
	}
	out, err := objects.HmacDigest(name, key, msg)
	if err != nil {
		return nil, hashlibUnsupportedError("unsupported hash type " + name)
	}
	return objects.NewBytes(out), nil
}

// hashlibPbkdf2 is _hashlib.pbkdf2_hmac(hash_name, password, salt, iterations,
// dklen=None).
func hashlibPbkdf2(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
	if len(pos) < 4 || len(pos) > 5 {
		return nil, objects.Raise(objects.TypeError, "pbkdf2_hmac() takes 4 or 5 positional arguments (%d given)", len(pos))
	}
	name, ok := objects.AsStr(pos[0])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "hash_name must be a string")
	}
	password, err := hashlibBytesArg(pos[1])
	if err != nil {
		return nil, err
	}
	salt, err := hashlibBytesArg(pos[2])
	if err != nil {
		return nil, err
	}
	iterations, ok := objects.AsInt(pos[3])
	if !ok {
		return nil, objects.Raise(objects.TypeError, "an integer is required")
	}
	dklen := 0
	dklenObj := objects.Object(objects.None)
	if len(pos) == 5 {
		dklenObj = pos[4]
	}
	for i, k := range kwNames {
		if k != "dklen" {
			return nil, objects.Raise(objects.TypeError, "pbkdf2_hmac() got an unexpected keyword argument '%s'", k)
		}
		dklenObj = kwVals[i]
	}
	if dklenObj != objects.None {
		v, iok := objects.AsInt(dklenObj)
		if !iok {
			return nil, objects.Raise(objects.TypeError, "an integer is required")
		}
		dklen = int(v)
	}
	out, err := objects.Pbkdf2Hmac(name, password, salt, int(iterations), dklen)
	if err != nil {
		return nil, err
	}
	return objects.NewBytes(out), nil
}
