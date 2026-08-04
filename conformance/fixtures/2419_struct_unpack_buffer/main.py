# struct.unpack, unpack_from and iter_unpack read any object that exposes the
# buffer protocol, not just bytes: a memoryview, an array.array and a bytearray
# all unpack the same as the equivalent bytes. unagi only accepted bytes and
# bytearray, so a memoryview or array raised a bytes-like TypeError where CPython
# reads the bytes behind the buffer. This pins buffer acceptance across the three
# read entry points and the Struct methods, that a wrong length still raises, and
# that a non-buffer (str, int, list) is still rejected.
import struct
import array

# unpack over a memoryview and an array, including a multi-byte typed array.
print("mv:", struct.unpack(">i", memoryview(b"\x00\x00\x00\x01")))
print("array b:", struct.unpack("4b", array.array("b", [1, 2, 3, 4])))
print("array i:", struct.unpack("<2i", array.array("i", [7, 9])))
print("bytearray:", struct.unpack(">i", bytearray(b"\x00\x00\x00\x02")))

# unpack_from over a buffer, at offset zero and a nonzero offset.
print("from mv:", struct.unpack_from(">h", memoryview(b"\x00\x05extra"), 0))
print("from mv off:", struct.unpack_from(">h", memoryview(b"ab\x00\x05"), 2))
print("from array off:", struct.unpack_from(">h", array.array("b", [0, 5, 0, 7]), 2))

# iter_unpack over a buffer yields the same records as over bytes.
print("iter mv:", list(struct.iter_unpack(">h", memoryview(b"\x00\x01\x00\x02"))))
print("iter array:", list(struct.iter_unpack("b", array.array("b", [1, 2, 3]))))

# The Struct methods take a buffer too.
s = struct.Struct(">i")
print("Struct.unpack mv:", s.unpack(memoryview(b"\x00\x00\x00\x03")))
print("Struct.iter_unpack array:", list(struct.Struct("b").iter_unpack(array.array("b", [9, 8]))))

# A buffer of the wrong length still raises struct.error.
try:
    struct.unpack(">i", memoryview(b"\x00\x01"))
except struct.error as e:
    print("wrong len:", e)

# A non-buffer object is still a bytes-like TypeError.
for bad in ("abcd", 42, [1, 2, 3, 4]):
    try:
        struct.unpack(">i", bad)
    except TypeError as e:
        print("reject", type(bad).__name__ + ":", e)
