# memoryview.suboffsets reports the pointer indirection of an indirect buffer
# (the PIL-style layout). Every view built from a bytes, bytearray, array or a
# cast/slice of one is a direct buffer over contiguous bytes, so the tuple is
# always empty, and a released view raises like the other metadata attributes.
import array


def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, str(ex))


show("bytes", lambda: memoryview(b"abc").suboffsets)
show("bytearray", lambda: memoryview(bytearray(8)).suboffsets)
show("array-i", lambda: memoryview(array.array("i", [1, 2, 3])).suboffsets)
show("cast-i", lambda: memoryview(bytearray(16)).cast("i").suboffsets)
show("cast-2d", lambda: memoryview(bytearray(12)).cast("B", shape=[3, 4]).suboffsets)
show("slice-step", lambda: memoryview(b"\x00\x01\x02\x03\x04")[::2].suboffsets)
show("readonly", lambda: memoryview(bytearray(b"ab")).toreadonly().suboffsets)

print("type", type(memoryview(b"abc").suboffsets).__name__)
print("hasattr", hasattr(memoryview(b"a"), "suboffsets"))

m = memoryview(bytearray(b"x"))
m.release()
show("released", lambda: m.suboffsets)
