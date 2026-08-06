from array import array


class MyInt(int):
    pass


class Idx:
    def __index__(self):
        return 3


class Bad:
    def __index__(self):
        return "no"


# Sequence repeat coerces the multiplier through __index__ the way CPython's
# sq_repeat does, so an int subclass repeats a str, bytes, list, tuple or array
# in either operand order.
n = MyInt(3)
print("ab" * n, n * "ab")
print([1, 2] * n, n * [1, 2])
print((1, 2) * n, n * (1, 2))
print(b"ab" * n, n * b"ab")
print(array("i", [1, 2]) * n, n * array("i", [1, 2]))

# A bare object spelling __index__ works everywhere an int multiplier does.
i = Idx()
print("xy" * i, i * "xy", [0] * i, (7,) * i, b"z" * i)

# A zero or negative count clamps to the empty sequence.
print(repr("ab" * MyInt(0)), repr("ab" * MyInt(-4)), [1] * MyInt(-1), b"x" * MyInt(0))

# Augmented repeat coerces the same way.
s = "ab"
s *= MyInt(2)
lst = [1]
lst *= Idx()
a = array("i", [9])
a *= MyInt(2)
print(s, lst, a)

# A list subclass repeats both directions off its payload.
class L(list):
    pass


print(L([1, 2]) * MyInt(2), MyInt(2) * L([3]), L([4]) * Idx())

# The bytes and bytearray count constructors coerce a count through __index__,
# so an int subclass or an __index__ object builds that many zero bytes.
print(bytes(MyInt(2)), bytearray(MyInt(2)), bytes(Idx()), bytearray(Idx()))

# A non-index multiplier still raises the sequence TypeError, and a bad __index__
# propagates from a repeat but is cleared to the cannot-convert error by the
# bytes constructor, matching CPython's two paths.
for label, expr in [
    ("str*float", lambda: "ab" * 1.5),
    ("list*str", lambda: [1] * "x"),
    ("tuple*None", lambda: (1,) * None),
    ("str*bad", lambda: "ab" * Bad()),
    ("list*bad", lambda: [1] * Bad()),
]:
    try:
        expr()
    except TypeError as e:
        print(label, e)
try:
    bytes(Bad())
except TypeError as e:
    print("bytes(bad)", e)
try:
    bytearray(Bad())
except TypeError as e:
    print("bytearray(bad)", e)

# A repeat count too large to index overflows, naming the operand's own type.
for label, expr in [
    ("str*big", lambda: "a" * MyInt(10 ** 30)),
    ("bytes(big)", lambda: bytes(MyInt(10 ** 30))),
    ("list*bigidx", lambda: [1] * MyInt(2 ** 80)),
]:
    try:
        expr()
    except OverflowError as e:
        print(label, e)
