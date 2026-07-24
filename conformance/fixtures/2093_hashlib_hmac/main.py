import hashlib
import hmac

algos = ["md5", "sha1", "sha224", "sha256", "sha384", "sha512",
         "sha3_224", "sha3_256", "sha3_384", "sha3_512"]
data = b"the quick brown fox"
for a in algos:
    h = hashlib.new(a)
    h.update(b"the quick ")
    h.update(b"brown fox")
    print(a, h.hexdigest(), h.name, h.digest_size, h.block_size)
    print(a, "digest", h.digest())

# direct constructors
print(hashlib.md5(data).hexdigest())
print(hashlib.sha256(data).hexdigest())
print(hashlib.sha512().hexdigest())

# shake extendable output
s1 = hashlib.shake_128(data)
print("shake128", s1.hexdigest(16), s1.hexdigest(32))
s2 = hashlib.shake_256(data)
print("shake256", s2.hexdigest(10))

# copy independence
h = hashlib.sha256(b"abc")
h2 = h.copy()
h.update(b"def")
print("orig", h.hexdigest())
print("copy", h2.hexdigest())

print("available", sorted(hashlib.algorithms_available & {"md5","sha1","sha256","sha512","sha3_256","shake_128","blake2b"}))
print("guaranteed", sorted(hashlib.algorithms_guaranteed))

# hmac
m = hmac.new(b"key", b"message", "sha256")
print("hmac name", m.hexdigest(), m.name, m.digest_size, m.block_size)
m2 = hmac.new(b"key", b"message", hashlib.sha256)
print("hmac mod", m2.hexdigest())
print("hmac digest", hmac.digest(b"key", b"message", "sha256"))
mm = hmac.new(b"key", digestmod="sha1")
mm.update(b"mes")
mm.update(b"sage")
print("hmac incr", mm.hexdigest())

# compare_digest
print("cmp eq", hmac.compare_digest(b"abc", b"abc"))
print("cmp ne", hmac.compare_digest(b"abc", b"abd"))
print("cmp len", hmac.compare_digest(b"abc", b"abcd"))
print("cmp str", hmac.compare_digest("abc", "abc"))

# pbkdf2
print("pbkdf2", hashlib.pbkdf2_hmac("sha256", b"password", b"salt", 1000).hex())
print("pbkdf2 dklen", hashlib.pbkdf2_hmac("sha1", b"pw", b"salt", 100, 20).hex())

# error and edge paths
try:
    hmac.new(b"key", b"msg")
except Exception as e:
    print(type(e).__name__, e)
try:
    hashlib.shake_128(b"x").hexdigest()
except Exception as e:
    print(type(e).__name__)
try:
    hashlib.new("nope")
except Exception as e:
    print(type(e).__name__)
try:
    hmac.compare_digest("abc", b"abc")
except Exception as e:
    print(type(e).__name__)
print(hashlib.sha256(bytearray(b"abc")).hexdigest())
print(hashlib.sha256(memoryview(b"abc")).hexdigest())
m = hmac.new(b"k", b"a", "sha256")
c = m.copy()
m.update(b"b")
print(m.hexdigest())
print(c.hexdigest())
