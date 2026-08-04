# struct.pack_into and struct.unpack_from validate their offset against the
# buffer before touching memory. A huge offset such as sys.maxsize used to
# overflow the internal offset+size sum and slip past the bounds check into a
# negative slice, so this pins the four distinct boundary errors CPython raises:
# a too-large offset, a shallow negative offset that leaves too little room, a
# deep negative offset that falls before the buffer start, and the ordinary
# too-small buffer. In-range positive and negative offsets keep working.
import struct
import sys


def show(label, fn):
    try:
        print(label, "->", fn())
    except struct.error as e:
        print(label, "-> struct.error:", e)
    except Exception as e:
        print(label, "->", type(e).__name__, ":", e)


fmt = "<I"

# A near-maxsize offset overflows a naive offset+size sum; the required-size
# number in the message is the true offset+4, well past a 64-bit int.
show("pack maxsize", lambda: struct.pack_into(fmt, bytearray(10), sys.maxsize, 1))
show("unpack maxsize", lambda: struct.unpack_from(fmt, bytes(10), sys.maxsize))
show("pack maxsize-1", lambda: struct.pack_into(fmt, bytearray(10), sys.maxsize - 1, 1))

# An ordinary too-large offset and a too-small buffer.
show("pack past end", lambda: struct.pack_into(fmt, bytearray(10), 8, 1))
show("unpack past end", lambda: struct.unpack_from(fmt, bytes(10), 8))
show("pack tiny buffer", lambda: struct.pack_into(fmt, bytearray(2), 0, 1))
show("unpack tiny buffer", lambda: struct.unpack_from(fmt, bytes(2), 0))

# A shallow negative offset leaves too little data for the format.
show("pack shallow neg", lambda: struct.pack_into(fmt, bytearray(10), -2, 1))
show("unpack shallow neg", lambda: struct.unpack_from(fmt, bytes(10), -2))

# A deep negative offset crosses the buffer start.
show("pack deep neg", lambda: struct.pack_into(fmt, bytearray(4), -8, 1))
show("unpack deep neg", lambda: struct.unpack_from(fmt, bytes(4), -8))
show("unpack -maxsize", lambda: struct.unpack_from(fmt, bytes(10), -sys.maxsize))

# In-range offsets still pack and unpack, including a valid negative offset.
buf = bytearray(8)
struct.pack_into(fmt, buf, 4, 0x01020304)
print("packed:", bytes(buf).hex())
print("unpack at 4:", struct.unpack_from(fmt, buf, 4))
print("unpack at -4:", struct.unpack_from(fmt, buf, -4))
print("unpack default offset:", struct.unpack_from(fmt, bytes(b"\x09\x00\x00\x00")))

# The same guards hold for a wider, natively aligned format.
show("pack wide past end", lambda: struct.pack_into("<Q", bytearray(4), 0, 1))
show("unpack wide maxsize", lambda: struct.unpack_from("<Q", bytes(16), sys.maxsize))
print("pack wide ok:", (lambda b: (struct.pack_into("<Q", b, 0, 0x1122334455667788), bytes(b).hex())[1])(bytearray(8)))
