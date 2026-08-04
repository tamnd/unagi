# struct.iter_unpack returns an unpack_iterator, a lazy cursor over a buffer, not
# a list. It yields one record tuple per call, is its own iterator, reports the
# number of records still to come through operator.length_hint, raises
# StopIteration once drained and stays drained, and its type cannot be
# constructed directly. This pins that surface for both the module function and
# the Struct method, alongside the empty-buffer and single-record cases.
import struct
import operator

s = struct.Struct(">IH")
buf = bytes([0, 0, 0, 1, 0, 2, 0, 0, 0, 3, 0, 4, 0, 0, 0, 0, 0, 0])

# The return value is an unpack_iterator.
it = s.iter_unpack(buf)
print("type:", type(it).__name__)

# length_hint counts down as records are drawn, and each record is a tuple.
print("hint:", operator.length_hint(it))
print(next(it), "hint:", operator.length_hint(it))
print(next(it), "hint:", operator.length_hint(it))
print(next(it), "hint:", operator.length_hint(it))

# Drained: StopIteration on the next draw and on every draw after that.
for _ in range(2):
    try:
        next(it)
    except StopIteration:
        print("StopIteration")
print("hint drained:", operator.length_hint(it))

# The module-level function behaves the same and is directly iterable.
print("module list:", list(struct.iter_unpack(">IH", buf)))

# An empty buffer yields no records at all.
empty = list(struct.iter_unpack(">IH", b""))
print("empty:", empty)

# A single record is one tuple then StopIteration.
one = struct.iter_unpack(">IH", bytes([0, 0, 0, 9, 0, 8]))
print("one:", next(one))
try:
    next(one)
except StopIteration:
    print("one drained")

# The iterator type is not constructible directly.
try:
    type(it)()
except TypeError as e:
    print("uninstantiable:", type(e).__name__)

# iter_unpack requires a buffer whose length is a multiple of the record size.
try:
    struct.iter_unpack(">IH", bytes(5))
except struct.error as e:
    print("bad length:", type(e).__name__)
