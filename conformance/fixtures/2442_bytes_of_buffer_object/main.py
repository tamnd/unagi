import array


def show(label, e):
    try:
        print(label, repr(e()))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, ex)


# bytes() and bytearray() of a memoryview or array copy the raw buffer the way
# CPython's buffer protocol does, so a typed view yields the whole itemsize bytes
# per element rather than one byte per element the iterable path would give. The
# result equals what tobytes() returns.
show("bytes-mv-i", lambda: bytes(memoryview(array.array("i", [7, 8, 3]))))
show("bytes-mv-h", lambda: bytes(memoryview(array.array("h", [1, 2]))))
show("bytes-mv-f", lambda: bytes(memoryview(array.array("f", [1.5]))))
show("bytes-mv-B", lambda: bytes(memoryview(b"abc")))
show("bytes-mv-cast", lambda: bytes(memoryview(bytearray(b"\x01\x00\x02\x00")).cast("h")))
show("bytes-mv-slice", lambda: bytes(memoryview(array.array("i", [1, 2, 3, 4]))[1:3]))
show("bytearray-mv-i", lambda: bytearray(memoryview(array.array("i", [7, 8]))))
show("bytes-array-i", lambda: bytes(array.array("i", [7, 8])))
show("bytes-array-h", lambda: bytes(array.array("h", [1, 2])))
show("bytearray-array-d", lambda: bytearray(array.array("d", [1.5])))

# The copy matches tobytes() and stays independent of the source, so a later
# mutation of the array does not touch the produced bytes.
a = array.array("i", [1, 2])
b = bytes(memoryview(a))
print("matches-tobytes", b == memoryview(a).tobytes())
a[0] = 99
print("independent", b == bytes(memoryview(array.array("i", [1, 2]))))

# A released memoryview forbids the buffer access, so bytes() of it is the
# released-memoryview ValueError rather than an empty result.
def released():
    m = memoryview(bytearray(b"abc"))
    m.release()
    return bytes(m)


show("bytes-released", released)

# The byte-format views and the plain iterable sources are unchanged: a 'B' view
# still reads its bytes and a list of ints still builds from its elements.
show("bytes-list", lambda: bytes([1, 2, 3]))
show("bytes-range", lambda: bytes(range(3)))
show("bytes-count", lambda: bytes(3))
