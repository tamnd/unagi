# secrets sits on random.SystemRandom, os.urandom and hmac.compare_digest, so it
# imports and runs once those are present. Every draw is nondeterministic, so the
# checks assert only structure, ranges, determinism of compare_digest and the
# error paths, never a specific random value.
import secrets
import string

# token_bytes returns bytes of the requested length, 32 by default.
tb = secrets.token_bytes(16)
print(type(tb).__name__, len(tb))
print(len(secrets.token_bytes()) == 32)

# token_hex is a hex string twice the byte length.
th = secrets.token_hex(16)
print(type(th).__name__, len(th), all(c in string.hexdigits for c in th))

# token_urlsafe returns a str at least as long as the byte count.
tu = secrets.token_urlsafe(16)
print(type(tu).__name__, len(tu) >= 16)

# randbelow stays in range, randbits fits the width.
print(all(0 <= secrets.randbelow(100) < 100 for _ in range(50)))
print(all(0 <= secrets.randbits(16) < 65536 for _ in range(50)))

# choice picks a member.
print(all(secrets.choice("abcdef") in "abcdef" for _ in range(50)))

# compare_digest is deterministic for bytes and str.
print(secrets.compare_digest(b"abc", b"abc"), secrets.compare_digest(b"abc", b"abd"))
print(secrets.compare_digest("xy", "xy"), secrets.compare_digest("xy", "xz"))

# SystemRandom is a random.Random flavour reading os.urandom.
sr = secrets.SystemRandom()
print(type(sr).__name__)
print(all(0 <= sr.randrange(10) < 10 for _ in range(50)))
print(sr.choice([1, 2, 3]) in [1, 2, 3])

# Distinct draws almost never collide.
print(len({secrets.token_hex(8) for _ in range(50)}) == 50)

# randbelow of a non-positive bound is the module's own ValueError.
try:
    secrets.randbelow(0)
except ValueError as e:
    print("randbelow(0):", e)
