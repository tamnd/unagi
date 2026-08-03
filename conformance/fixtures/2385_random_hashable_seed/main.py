# _random.Random seeds from the hash of a non-int seed, cast to an unsigned
# size_t, so a float seed through the public class and any hashable seed through
# the accelerator is reproducible. str/bytes seeds ride CPython's randomized str
# hash on the raw accelerator so they are left out here; random.py hashes them
# through sha512 on its own path.
import random
import _random


def stream(seed, n=4):
    r = random.Random(seed)
    return [r.random() for _ in range(n)]


def raw(seed, n=4):
    r = _random.Random(seed)
    return [r.random() for _ in range(n)]


# A float seed through random.Random is stable and matches an int seed keyed on
# its hash.
print(stream(3.5))
print(stream(2.5))
print(stream(-1.5))
print(stream(0.0))
print(stream(3.5) == stream(hash(3.5)))
# bool is an int subtype: True seeds like 1, False like 0.
print(stream(True) == stream(1))
print(stream(False) == stream(0))

# The accelerator accepts any hashable seed directly, including a tuple, and
# seeds from its hash.
print(raw(3.5) == stream(3.5))
print(raw((1, 2)) == raw((1, 2)))
print(raw((1, 2)) != raw((1, 3)))
print(raw((1, 2)) == raw(hash((1, 2))))
print(_random.Random(hash(3.5)).random() == _random.Random(3.5).random())

# An unhashable seed surfaces the TypeError hashing raises.
for bad in ([1, 2], {1: 2}, {1, 2}):
    try:
        _random.Random(bad)
    except TypeError as e:
        print(type(e).__name__, str(e))

# random.py still rejects the genuinely unsupported types with its own message
# before it ever reaches the accelerator.
for bad in (complex(1, 2), (1, 2)):
    try:
        random.Random(bad)
    except TypeError as e:
        print(str(e))
