import struct


def show(label, fn):
    try:
        print(label, "->", fn())
    except Exception as e:
        print(label, "|", type(e).__name__, "|", e)


buf = bytearray(8)
s = struct.Struct(">i")

# Module function argument-count errors.
show("pack0", lambda: struct.pack())
show("unpack3", lambda: struct.unpack("i", b"abcd", 1))
show("unpack1", lambda: struct.unpack("i"))
show("unpack0", lambda: struct.unpack())
show("calcsize0", lambda: struct.calcsize())
show("calcsize2", lambda: struct.calcsize("i", "j"))
show("iter_unpack1", lambda: struct.iter_unpack("i"))
show("iter_unpack3", lambda: struct.iter_unpack("i", b"abcd", 1))
show("pack_into0", lambda: struct.pack_into())
show("pack_into1", lambda: struct.pack_into("i"))
show("pack_into2", lambda: struct.pack_into("i", buf))
show("unpack_from0", lambda: struct.unpack_from())
show("unpack_from1", lambda: struct.unpack_from("i"))
show("unpack_from4", lambda: struct.unpack_from(">i", b"abcd", 0, 9))

# Struct method argument-count errors.
show("s.unpack0", lambda: s.unpack())
show("s.unpack2", lambda: s.unpack(b"abcd", 1))
show("s.iter_unpack0", lambda: s.iter_unpack())
show("s.iter_unpack2", lambda: s.iter_unpack(b"abcd", 1))
show("s.pack_into0", lambda: s.pack_into())
show("s.pack_into1", lambda: s.pack_into(buf))
show("s.unpack_from0", lambda: s.unpack_from())
show("s.unpack_from3", lambda: s.unpack_from(b"abcd", 0, 9))

# Keyword rejection carries the qualified name.
show("pack_kw", lambda: struct.pack(format="i"))
show("unpack_kw", lambda: struct.unpack("i", buffer=b"abcd"))
show("calcsize_kw", lambda: struct.calcsize(format="i"))
show("iter_unpack_kw", lambda: struct.iter_unpack("i", buffer=b"abcd"))
show("pack_into_kw", lambda: struct.pack_into("i", buffer=buf, offset=0))
show("s.unpack_kw", lambda: s.unpack(buffer=b"abcd"))

# unpack_from accepts buffer and offset by keyword.
show("uf_kwbuf", lambda: struct.unpack_from(">i", buffer=b"abcd"))
show("uf_kwoff", lambda: struct.unpack_from(">i", b"\x00abcd", offset=1))
show("uf_kwboth", lambda: struct.unpack_from(">i", buffer=b"\x00abcd", offset=1))
show("s.uf_kwbuf", lambda: s.unpack_from(buffer=b"abcd"))
show("s.uf_kwboth", lambda: s.unpack_from(buffer=b"\x00abcd", offset=1))

# unpack_from keyword edge cases.
show("uf_dupbuf", lambda: struct.unpack_from(">i", b"abcd", buffer=b"abcd"))
show("uf_badkw", lambda: struct.unpack_from(">i", b"abcd", bogus=1))
show("uf_badoff", lambda: struct.unpack_from(">i", b"abcd", "x"))
show("uf_offkw_badtype", lambda: struct.unpack_from(">i", b"abcd", offset="x"))
show("s.uf_dupbuf", lambda: s.unpack_from(b"abcd", buffer=b"abcd"))

# The valid positional forms still work.
show("pack_ok", lambda: struct.pack(">i", 1).hex())
show("unpack_ok", lambda: struct.unpack(">i", b"\x00\x00\x00\x05"))
show("unpack_from_ok", lambda: struct.unpack_from(">i", b"zz\x00\x00\x00\x07", 2))
show("pack_into_ok", lambda: (struct.pack_into(">i", buf, 0, 9), buf.hex())[1])
