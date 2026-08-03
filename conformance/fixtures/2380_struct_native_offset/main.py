import struct


def show(label, fn):
    try:
        print(label, "->", fn())
    except Exception as e:
        print(label, "|", type(e).__name__, "|", e)


# Native single-item unpack_from at every offset alignment. CPython measures a
# format's alignment from the struct's own start, not from the buffer offset,
# so the read lands right at the given offset regardless of its remainder.
for off in range(0, 5):
    b = bytes(off) + b"\x00\x00\x00\x05" + b"\xff\xff"
    show("uf_i_off%d" % off, lambda b=b, off=off: struct.unpack_from("i", b, off))

# Native compound with internal padding: b at 0, pad to 4, i at 4, size 8.
compound = b"zz" + b"\x01" + b"\x00\x00\x00" + b"\x00\x00\x00\x07"
show("calcsize_bi", lambda: struct.calcsize("bi"))
show("uf_bi_off0", lambda: struct.unpack_from("bi", b"\x01\x00\x00\x00\x00\x00\x00\x07", 0))
show("uf_bi_off2", lambda: struct.unpack_from("bi", compound, 2))

# Native offset through the Struct method path.
s = struct.Struct("hi")
show("calcsize_hi", lambda: s.size)
buf = b"aa" + b"\x02\x00" + b"\x00\x00" + b"\x09\x00\x00\x00"
show("s.uf_hi_off2", lambda: s.unpack_from(buf, 2))
show("s.uf_hi_kwoff", lambda: s.unpack_from(buffer=buf, offset=2))

# Native iter_unpack walks record by record, each record aligned from its own
# start, so the second record unpacks the same way as the first.
recs = b"\x01\x00\x00\x00\x00\x00\x00\x07\x02\x00\x00\x00\x00\x00\x00\x08"
show("iter_bi", lambda: list(struct.iter_unpack("bi", recs)))

# Native pack_into at a nonzero offset lays the item down aligned from the
# offset, matching CPython, and leaves the surrounding bytes untouched.
def pack_native(fmt, off, *vals):
    dst = bytearray(b"\xaa" * 16)
    struct.pack_into(fmt, dst, off, *vals)
    return dst.hex()


show("pi_i_off1", lambda: pack_native("i", 1, 5))
show("pi_bi_off2", lambda: pack_native("bi", 2, 1, 7))
show("s.pi_hi_off3", lambda: (lambda dst: (struct.Struct("hi").pack_into(dst, 3, 2, 9), dst.hex())[1])(bytearray(b"\xaa" * 16)))

# The offset bound checks still fire on a native format.
show("uf_short", lambda: struct.unpack_from("i", b"\x00\x00", 0))
show("uf_neg", lambda: struct.unpack_from("i", b"\x00\x00\x00\x05", -4))
show("uf_off_oob", lambda: struct.unpack_from("i", b"\x00\x00\x00\x05", 1))
