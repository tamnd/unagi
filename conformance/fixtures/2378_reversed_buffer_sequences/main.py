import array

samples = [
    ("array_i", array.array("i", [10, 20, 30])),
    ("array_d", array.array("d", [1.5, 2.5])),
    ("array_b", array.array("b", [1, 2, 3])),
    ("array_empty", array.array("i")),
    ("bytes", b"abc"),
    ("bytes_empty", b""),
    ("bytearray", bytearray(b"xyz")),
    ("memoryview", memoryview(b"\x01\x02\x03")),
]
for name, o in samples:
    r = reversed(o)
    print(name, type(r).__name__, iter(r) is r, list(r))

# The reversed iterator drains item by item and then stops.
r = reversed(b"hi")
print(next(r), next(r))
try:
    next(r)
except StopIteration:
    print("stop")

# Reversing a bytearray does not consume or alter the source.
ba = bytearray(b"abc")
list(reversed(ba))
print(bytes(ba))

