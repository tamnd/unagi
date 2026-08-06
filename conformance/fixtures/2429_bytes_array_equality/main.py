from array import array


def show(label, v):
    print(label, v)


ba = bytearray(b"ab")
bs = b"ab"
ar = array("b", [97, 98])
mv = memoryview(b"ab")

# The full pairwise table across the four buffer types with equal content. The
# only inequality CPython reports is bytes against an array, in either order,
# every other pairing compares equal both ways.
pairs = {
    "bytes,bytes": (bs, b"ab"),
    "bytes,bytearray": (bs, ba),
    "bytes,array": (bs, ar),
    "bytes,mv": (bs, mv),
    "bytearray,array": (ba, ar),
    "bytearray,mv": (ba, mv),
    "array,mv": (ar, mv),
    "array,array": (ar, array("b", [97, 98])),
    "mv,mv": (mv, memoryview(b"ab")),
}
for k, (a, b) in pairs.items():
    show(k, (a == b, b == a, a != b, b != a))

# Unequal content is unequal across every pairing.
show("neq-bytes-array", b"ax" == array("b", [97, 98]))
show("neq-array-ba", array("b", [97, 99]) == bytearray(b"ab"))
show("neq-array-mv", array("b", [1, 2]) == memoryview(b"ab"))

# A wider element width changes the raw bytes, so an int-typecode array does not
# read as the two ascii bytes, so a bytearray of those bytes is unequal.
show("int-array-vs-ba", array("i", [97, 98]) == bytearray(b"ab"))

# A bytes still keys a dict and a set the same way, unaffected by the array rule.
d = {b"ab": 1}
show("dict-hit", d.get(b"ab"))
show("set-in", b"ab" in {b"ab", b"cd"})

# A bytes subclass on the right still reads as its bytes.
class B(bytes):
    pass


show("bytes-vs-subclass", b"ab" == B(b"ab"))
show("array-vs-bytes-subclass", array("b", [97, 98]) == B(b"ab"))
