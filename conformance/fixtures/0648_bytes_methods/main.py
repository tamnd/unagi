# The read-method surface shared by bytes and bytearray: search, predicates
# and hex rendering. Every value below prints identically under CPython 3.14.
b = b"abcabcabc"

# count with an int byte, a bytes-like needle and a window.
print(b.count(b"a"), b.count(b"bc"), b.count(97), b.count(b""))
print(b.count(b"a", 1), b.count(b"a", 1, 7), b.count(b"", 1, 2))

# find/rfind return the offset or -1; index/rindex raise when absent.
print(b.find(b"c"), b.rfind(b"c"), b.find(b"z"), b.find(97))
print(b.find(b"a", 1), b.find(b"a", 1, 2), b.rfind(b"a", 0, 5))
print(b.index(b"c"), b.rindex(b"c"))
print(b"abc".find(b""), b"abc".find(b"", 3), b"abc".find(b"", 4))
print(b"abc".rfind(b""), b"abc".rfind(b"", 0, 2))

# startswith/endswith take a fix or a tuple of fixes and an optional window.
print(b.startswith(b"ab"), b.endswith(b"bc"))
print(b.startswith((b"x", b"ab")), b.startswith(b"bc", 1))
print(b.endswith(b"ab", 0, 5), b"abc".endswith(b""))

# hex rendering, with and without separators.
print(b"abc".hex(), b"".hex())
print(b"abc".hex("-"), b"abc".hex("_", 2))
print(b"\x01\x02\x03\x04\x05".hex(":", 2), b"\x01\x02\x03\x04\x05".hex(":", -2))
print(b"ab".hex(b"-"))

# bytearray mirrors the same surface on a mutable receiver.
ba = bytearray(b"abcabc")
print(ba.count(b"a"), ba.find(b"c"), ba.startswith(b"ab"), ba.hex("-"))

# Catchable errors, one per labelled line.
try:
    b.index(b"z")
except ValueError as e:
    print("index", e)
try:
    b.count("a")
except TypeError as e:
    print("counttype", e)
try:
    b.find(2.0)
except TypeError as e:
    print("findfloat", e)
try:
    b.count(300)
except ValueError as e:
    print("countrange", e)
try:
    b.startswith("a")
except TypeError as e:
    print("swtype", e)
try:
    b.startswith((b"x", 5))
except TypeError as e:
    print("swtuple", e)
try:
    b.find(b"a", "x")
except TypeError as e:
    print("startstr", e)
try:
    b"ab".hex("--")
except ValueError as e:
    print("seplen", e)
try:
    b"ab".hex(1)
except TypeError as e:
    print("sepint", e)
