import struct


def show(label, e):
    try:
        print(label, repr(e()))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, str(ex))


class Idx:
    def __index__(self):
        return 2


class BadIdx:
    def __index__(self):
        return "x"


# struct packs an integer field through __index__ the way CPython feeds a field
# to PyNumber_Index, so an object spelling __index__ packs as its value across the
# signed and unsigned widths and a bool counts as 0 or 1. A bad __index__ return
# propagates its non-int TypeError, and a value with no __index__ (a float) is the
# struct not-an-integer error.
show("pack-h", lambda: struct.pack(">h", Idx()))
show("pack-B", lambda: struct.pack(">B", Idx()))
show("pack-q", lambda: struct.pack(">q", Idx()))
show("pack-bool-field", lambda: struct.pack(">b", True))
show("pack-multi", lambda: struct.pack(">hh", Idx(), Idx()))
show("pack-bad", lambda: struct.pack(">h", BadIdx()))
show("pack-float", lambda: struct.pack(">h", 1.5))
show("pack-str", lambda: struct.pack(">h", "x"))
show("pack-range", lambda: struct.pack(">b", 200))

# The offset argument to unpack_from, pack_into and the Struct methods runs
# through __index__ the same way, so an __index__ offset reads or writes at its
# value, a bad __index__ return propagates and a float or str offset is the
# cannot-be-interpreted TypeError.
show("unpack-from-idx", lambda: struct.unpack_from(">b", b"zz\x05", Idx()))
show("unpack-from-bad", lambda: struct.unpack_from(">b", b"zz\x05", BadIdx()))
show("unpack-from-float", lambda: struct.unpack_from(">b", b"zz\x05", 1.5))
show("struct-unpack-from-idx", lambda: struct.Struct(">b").unpack_from(b"zz\x05", Idx()))


def pack_into_idx():
    b = bytearray(4)
    struct.pack_into(">h", b, Idx(), 7)
    return bytes(b)


def pack_into_bad():
    b = bytearray(4)
    struct.pack_into(">h", b, BadIdx(), 7)
    return bytes(b)


def pack_into_float():
    b = bytearray(4)
    struct.pack_into(">h", b, 1.5, 7)
    return bytes(b)


def struct_pack_into_idx():
    b = bytearray(4)
    struct.Struct(">h").pack_into(b, Idx(), 7)
    return bytes(b)


show("pack-into-idx", pack_into_idx)
show("pack-into-bad", pack_into_bad)
show("pack-into-float", pack_into_float)
show("struct-pack-into-idx", struct_pack_into_idx)
