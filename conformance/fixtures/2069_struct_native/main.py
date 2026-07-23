# _struct is the C accelerator the struct module re-exports wholesale:
# struct.py is `from _struct import *` and nothing else, so pack, unpack,
# calcsize, the Struct class and the error exception all live here. base64
# reaches it through `struct.Struct('!I')`, so struct has to work end to end.
#
# Every format here pins an explicit byte order (<, >, =, !) so the packed
# bytes are identical on every host regardless of native sizes or endianness.

import _struct

# The whole public surface is present.
_names = ["Struct", "_clearcache", "calcsize", "error", "iter_unpack",
          "pack", "pack_into", "unpack", "unpack_from"]
print("attrs", [n for n in _names if hasattr(_struct, n)])

import struct

# calcsize sums the standard sizes; a count binds to the following code and
# whitespace between codes is ignored.
print("calcsize", struct.calcsize("<hId"), struct.calcsize("<3h 2i"), struct.calcsize(""))

# pack and unpack round-trip integers, floats, bytes and bools across orders.
print("pack le", struct.pack("<hId", 1, 2, 3.5).hex())
print("pack be", struct.pack(">hId", 1, 2, 3.5).hex())
print("unpack", struct.unpack("<hId", struct.pack("<hId", 1, 2, 3.5)))
print("signed", struct.pack(">bh", -1, 258).hex(), struct.unpack(">bh", b"\xff\x01\x02"))
print("wide", struct.pack("<qQ", -2, 2 ** 63).hex())
print("net", struct.pack("!I", 65535).hex())
print("bool", struct.pack("<?", True).hex(), struct.unpack("<?", b"\x00"))
print("half", struct.pack("<e", 1.5).hex(), struct.unpack("<e", b"\x00\x3c"))
print("floats", struct.pack("<fd", 1.5, 2.5).hex())

# s is a raw fixed field (truncated, zero-padded); c is one byte; p is a
# length-prefixed pascal string.
print("s", struct.pack("<4s", b"hi").hex(), struct.unpack("<4s", b"hi\x00\x00"))
print("c", struct.pack("<3c", b"a", b"b", b"c"))
print("p", struct.pack("<5p", b"abcd").hex(), struct.unpack("<5p", struct.pack("<5p", b"abcdef")))

# The Struct class caches a format and exposes size and format.
s = struct.Struct("!I")
print("Struct", s.size, s.format, s.pack(65535).hex(), s.unpack(b"\x00\x00\xff\xff"))

# pack_into writes into a bytearray at an offset; unpack_from reads back.
buf = bytearray(8)
struct.pack_into("<hh", buf, 2, 5, 6)
print("pack_into", buf.hex(), struct.unpack_from("<hh", buf, 2))

# iter_unpack walks a buffer record by record.
print("iter", list(struct.iter_unpack("<h", b"\x01\x00\x02\x00\x03\x00")))

# The error exception is a catchable Exception subclass, which base64 relies on.
print("error is Exception", issubclass(struct.error, Exception))


def err(fn):
    try:
        fn()
        print("NOERR")
    except struct.error as e:
        print("struct.error", e)


err(lambda: struct.pack("<I", -1))
err(lambda: struct.pack("<b", 200))
err(lambda: struct.pack("<hh", 1))
err(lambda: struct.unpack("<I", b"\x00"))
err(lambda: struct.calcsize("<n"))
err(lambda: struct.calcsize("z"))
err(lambda: struct.unpack_from("<h", b"ab", 5))
err(lambda: list(struct.iter_unpack("<h", b"\x00")))
