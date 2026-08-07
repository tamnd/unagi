import array


def show(label, e):
    try:
        print(label, repr(e()))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, ex)


# to_bytes runs its length through __index__ the way CPython feeds the argument to
# PyNumber_Index, so an object spelling __index__ counts as its integer width and
# a bool works as 0 or 1. A bad __index__ return keeps the non-int TypeError and a
# non-integer with no __index__ is the cannot-be-interpreted TypeError.
class Idx:
    def __index__(self):
        return 3


class BadIdx:
    def __index__(self):
        return "x"


show("tb-idx-pos", lambda: (5).to_bytes(Idx(), "big"))
show("tb-idx-kw", lambda: (5).to_bytes(length=Idx(), byteorder="big"))
show("tb-bool-len", lambda: (1).to_bytes(True, "big"))
show("tb-bad-idx", lambda: (5).to_bytes(BadIdx(), "big"))
show("tb-float-len", lambda: (5).to_bytes(1.5, "big"))

# The byteorder type error names the calling method, so a non-str byteorder on
# to_bytes reads to_bytes() while the same slip on from_bytes reads from_bytes().
show("tb-order-idx", lambda: (5).to_bytes(3, Idx()))
show("tb-order-int", lambda: (5).to_bytes(3, 2))
show("fb-order-int", lambda: int.from_bytes(b"\x05", 2))

# from_bytes reads a memoryview or array through the buffer protocol, so a typed
# view yields its whole itemsize bytes rather than one integer per element, and a
# plain byte view still reads its bytes.
show("fb-mv-i", lambda: int.from_bytes(memoryview(array.array("i", [1])), "big"))
show("fb-array-i", lambda: int.from_bytes(array.array("i", [1]), "big"))
show("fb-mv-h", lambda: int.from_bytes(memoryview(array.array("h", [258])), "little"))
show("fb-mv-B", lambda: int.from_bytes(memoryview(b"\x01\x02"), "big"))

# A released view forbids the buffer access with the released-memoryview
# ValueError rather than reading empty bytes.
def fb_released():
    m = memoryview(bytearray(b"\x01\x02"))
    m.release()
    return int.from_bytes(m, "big")


show("fb-released", fb_released)

# A str source is the cannot-convert TypeError, matching CPython's buffer
# conversion which special-cases str, while a list of ints and a bytes value still
# build normally and a plain int is the cannot-convert TypeError.
show("fb-str", lambda: int.from_bytes("ab", "big"))
show("fb-list", lambda: int.from_bytes([1, 2], "big"))
show("fb-bytes", lambda: int.from_bytes(b"\x01\x02", "big"))
show("fb-int", lambda: int.from_bytes(5, "big"))
show("fb-float", lambda: int.from_bytes(1.5, "big"))
show("fb-list-oob", lambda: int.from_bytes([256], "big"))

# The round trip still holds for the ordinary forms, signed and unsigned.
show("rt-unsigned", lambda: int.from_bytes((258).to_bytes(2, "big"), "big"))
show("rt-signed", lambda: int.from_bytes((-1).to_bytes(2, "big", signed=True), "big", signed=True))
