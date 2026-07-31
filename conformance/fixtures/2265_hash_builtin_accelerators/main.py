"""The builtin (non-OpenSSL) hash accelerators _md5, _sha1, _sha2, _sha3 back
hashlib's __get_builtin_constructor fallback. Each exposes named constructors
returning the same HASH object (update/digest/hexdigest/copy) hashlib uses."""

import _md5
import _sha1
import _sha2
import _sha3
import hashlib

msg = b"The quick brown fox"

# Direct construction through each builtin module, one-shot digests.
print("md5     :", _md5.md5(msg).hexdigest())
print("sha1    :", _sha1.sha1(msg).hexdigest())
print("sha224  :", _sha2.sha224(msg).hexdigest())
print("sha256  :", _sha2.sha256(msg).hexdigest())
print("sha384  :", _sha2.sha384(msg).hexdigest())
print("sha512  :", _sha2.sha512(msg).hexdigest())
print("sha3_256:", _sha3.sha3_256(msg).hexdigest())
print("sha3_512:", _sha3.sha3_512(msg).hexdigest())

# The extendable-output shakes take a length.
print("shake128:", _sha3.shake_128(msg).hexdigest(16))
print("shake256:", _sha3.shake_256(msg).hexdigest(32))

# Object metadata mirrors CPython.
h = _sha2.sha256()
print("meta    :", h.name, h.digest_size, h.block_size)
print("empty   :", _sha2.sha256().hexdigest())

# Incremental feeding matches one-shot, and copy() forks the state.
h = _sha2.sha256()
h.update(b"The quick ")
fork = h.copy()
h.update(b"brown fox")
print("chunked :", h.hexdigest())
print("fork    :", fork.hexdigest())
print("fork==sq:", fork.hexdigest() == _sha2.sha256(b"The quick ").hexdigest())

# digest() returns raw bytes whose length is digest_size.
d = _sha1.sha1(msg).digest()
print("digest  :", type(d).__name__, len(d))

# hashlib's own builtin-constructor path resolves to these modules and agrees.
bmd5 = hashlib.__get_builtin_constructor("md5")
print("via hashlib builtin:", bmd5(msg).hexdigest() == _md5.md5(msg).hexdigest())

# usedforsecurity is accepted and ignored; the digest is unchanged.
print("ufs     :", _md5.md5(msg, usedforsecurity=False).hexdigest())
