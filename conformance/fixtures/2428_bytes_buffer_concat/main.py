from array import array


def iadd(x, y):
    x += y
    return x


# bytes and bytearray concatenation accepts any bytes-like right operand through
# the buffer protocol, a memoryview or array as well as a bytes or bytearray, and
# the result keeps the left operand's type.
mv = memoryview(b"cd")
ar = array("b", [99, 100])
print(b"ab" + mv, b"ab" + ar, b"ab" + bytearray(b"cd"), b"ab" + b"cd")
print(bytearray(b"ab") + mv, bytearray(b"ab") + ar, bytearray(b"ab") + b"cd")
print(type(b"ab" + mv).__name__, type(bytearray(b"ab") + mv).__name__)

# A memoryview over a sliced or larger buffer contributes exactly its span.
big = memoryview(b"wxyz")[1:3]
print(b"[" + big + b"]", bytearray(b"[") + big)

# A bytes subclass reads as its underlying bytes on the right the way it already did.
class B(bytes):
    pass


print(b"ab" + B(b"cd"), bytearray(b"ab") + B(b"cd"))

# In place += accepts the same buffer operands and mutates in place, so aliases
# see the growth and the object identity is unchanged.
ba = bytearray(b"ab")
alias = ba
ba += mv
ba += ar
ba += b"ef"
print(ba, alias is ba, alias)

# A non-buffer right operand still raises the concat TypeError unchanged, for both
# the binary and the in place forms.
for label, expr in [
    ("bytes+str", lambda: b"ab" + "cd"),
    ("bytes+list", lambda: b"ab" + [1]),
    ("bytes+int", lambda: b"ab" + 5),
    ("ba+str", lambda: bytearray(b"ab") + "cd"),
    ("ba+None", lambda: bytearray(b"ab") + None),
    ("ba+=list", lambda: iadd(bytearray(b"ab"), [1])),
    ("ba+=str", lambda: iadd(bytearray(b"ab"), "z")),
]:
    try:
        expr()
    except TypeError as e:
        print(label, e)

# A memoryview left operand has no __add__, so it still declines, unchanged.
try:
    mv + b"cd"
except TypeError as e:
    print("mv+bytes", e)
