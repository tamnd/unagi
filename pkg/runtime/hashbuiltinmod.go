package runtime

import (
	"github.com/tamnd/unagi/pkg/objects"
)

// _md5, _sha1, _sha2 and _sha3 are the builtin (non-OpenSSL) hash accelerators.
// hashlib.py's __get_builtin_constructor imports whichever of them it needs and
// binds their constructors -- _md5.md5, _sha1.sha1, _sha2.sha256, _sha3.sha3_256
// and so on -- as the hashlib.<name> the program calls. They are the fallback
// path when _hashlib (OpenSSL) is absent or has an algorithm disabled by policy,
// and test_hashlib exercises each builtin module directly through
// _conditional_import_module. CPython always compiles these under POSIX, so a
// missing one makes test_hashlib log a "C extension failed to compile" warning
// rather than skip; registering them keeps that path quiet and correct.
//
// The digest engines and the HASH object (update/digest/hexdigest/copy) already
// live in pkg/objects behind NewHashByName, shared with _hashlib, so these
// modules are only the constructor surface. Each constructor takes the same
// (data=b'', *, usedforsecurity=True) shape as the openssl_<name> ones, parsed
// by hashlibConstructorArgs; usedforsecurity is accepted and ignored.

func init() {
	moduleTable["_md5"] = &moduleEntry{builtin: true, exec: hashBuiltinInit([]string{"md5"})}
	moduleTable["_sha1"] = &moduleEntry{builtin: true, exec: hashBuiltinInit([]string{"sha1"})}
	moduleTable["_sha2"] = &moduleEntry{builtin: true, exec: hashBuiltinInit(
		[]string{"sha224", "sha256", "sha384", "sha512"})}
	moduleTable["_sha3"] = &moduleEntry{builtin: true, exec: hashBuiltinInit(
		[]string{"sha3_224", "sha3_256", "sha3_384", "sha3_512", "shake_128", "shake_256"})}
}

// hashBuiltinInit builds the exec for a builtin hash module, registering one
// constructor per algorithm name it provides. Each constructor is keyed on the
// algorithm so hashlib and test_hashlib get the same callable type across modules.
func hashBuiltinInit(algos []string) func(*objects.Module) error {
	return func(m *objects.Module) error {
		for _, name := range algos {
			n := name
			fn := objects.NewFuncKw(n, func(pos []objects.Object, kwNames []string, kwVals []objects.Object) (objects.Object, error) {
				data, err := hashlibConstructorArgs(n, pos, kwNames)
				if err != nil {
					return nil, err
				}
				return objects.NewHashByName(n, data)
			})
			if err := objects.StoreAttr(m, n, fn); err != nil {
				return err
			}
		}
		return nil
	}
}
