import struct
import array


def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# pack_into writes into any read-write buffer CPython accepts, not just a
# bytearray: a memoryview over a bytearray or an array, and an array.array
# directly. The buffer is mutated in place, so its bytes read back the packed
# string.
def into_bytearray():
    ba = bytearray(b" " * 10)
    struct.pack_into("4s", ba, 2, b"abcd")
    return bytes(ba)


def into_array():
    a = array.array("b", b" " * 10)
    struct.pack_into("4s", a, 0, b"abcd")
    return a.tobytes()


def into_mv_bytearray():
    mv = memoryview(bytearray(b" " * 10))
    struct.pack_into("4s", mv, 3, b"abcd")
    return mv.tobytes()


def into_mv_array():
    mv = memoryview(array.array("b", b" " * 10))
    struct.pack_into("4s", mv, 1, b"abcd")
    return mv.tobytes()


show("bytearray", into_bytearray)
show("array", into_array)
show("mv over bytearray", into_mv_bytearray)
show("mv over array", into_mv_array)

# A negative offset counts from the end of the buffer.
def negative_offset():
    a = array.array("b", b" " * 20)
    struct.pack_into("4s", a, -6, b"abcd")
    return a.tobytes()


show("negative offset", negative_offset)

# A read-only or non-buffer destination is the TypeError naming its type, with
# None spelled "None" rather than its NoneType.
for dst in [b" " * 10, " " * 10, [0] * 10, None, 5, (1, 2)]:
    show("bad %r" % (dst,), (lambda d: (lambda: struct.pack_into("4s", d, 0, b"abcd")))(dst))

# A strided or reversed memoryview is not a contiguous writable buffer.
show("mv strided", lambda: struct.pack_into("4s", memoryview(bytearray(b" " * 20))[::2], 0, b"abcd"))
show("mv reversed", lambda: struct.pack_into("4s", memoryview(bytearray(b" " * 20))[::-1], 0, b"abcd"))
# A read-only memoryview over bytes declines too.
show("mv readonly", lambda: struct.pack_into("4s", memoryview(b" " * 10), 0, b"abcd"))

# An offset past the machine range is an IndexError, and a non-integer offset is
# the not-an-integer TypeError.
ba = bytearray(b" " * 10)
show("offset 2**1000", lambda: struct.pack_into("4s", ba, 2 ** 1000, b"abcd"))
show("offset -2**1000", lambda: struct.pack_into("4s", ba, -(2 ** 1000), b"abcd"))
show("offset float", lambda: struct.pack_into("4s", ba, 0.0, b"abcd"))
show("offset None", lambda: struct.pack_into("4s", ba, None, b"abcd"))
