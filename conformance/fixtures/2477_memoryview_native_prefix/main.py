import struct


def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# The optional '@' native-order prefix casts to the native single code, and the
# view keeps the '@i' spelling in its format while sizing and decoding the bare
# code.
src = struct.pack("<2i", 7, -3)
show("at_i_format", lambda: memoryview(src).cast("@i").format)
show("at_i_itemsize", lambda: memoryview(src).cast("@i").itemsize)
show("at_i_tolist", lambda: memoryview(src).cast("@i").tolist())
show("at_B_format", lambda: memoryview(b"AB").cast("@B").format)
show("at_d_tolist", lambda: memoryview(struct.pack("<d", 1.5)).cast("@d").tolist())
show("at_bool_tolist", lambda: memoryview(bytes([0, 1])).cast("@?").tolist())
show("at_c_tolist", lambda: memoryview(b"AB").cast("@c").tolist())
show("at_e_tolist", lambda: memoryview(struct.pack("<e", 1.5)).cast("@e").tolist())

# The prefix travels with a slice and with toreadonly.
at = memoryview(struct.pack("<4i", 1, 2, 3, 4)).cast("@i")
show("slice_format", lambda: at[1:3].format)
show("slice_tolist", lambda: at[1:3].tolist())
show("toreadonly_format", lambda: at.toreadonly().format)

# A native view compares equal to the standard view of the same bytes, since the
# element values match.
show("eq_at_plain", lambda: memoryview(src).cast("@i") == memoryview(src).cast("i"))
show("eq_at_at", lambda: memoryview(src).cast("@i") == memoryview(src).cast("@i"))

# Contains and count read the decoded values the same as the bare code.
show("contains", lambda: 7 in memoryview(src).cast("@i"))

# A native byte view hashes to the same value as its bytes, while a native wide
# view is still restricted to the byte formats.
show("hash_c_eq_bytes", lambda: (lambda v: hash(v) == hash(v.tobytes()))(memoryview(b"AB").cast("@c")))
show("hash_i", lambda: hash(memoryview(struct.pack("<i", 5)).cast("@i")))

# A store through a native view names the bare code in its errors.
show("write_i_ok", lambda: (lambda b: (memoryview(b).cast("@i").__setitem__(0, 42), struct.unpack("<i", b)[0])[1])(bytearray(4)))
show("write_i_str", lambda: memoryview(bytearray(4)).cast("@i").__setitem__(0, "x"))
show("write_i_over", lambda: memoryview(bytearray(4)).cast("@i").__setitem__(0, 2 ** 31))
show("write_c_int", lambda: memoryview(bytearray(2)).cast("@c").__setitem__(0, 5))
show("write_e_over", lambda: memoryview(bytearray(2)).cast("@e").__setitem__(0, 1e5))

# The prefix takes exactly one native code, so a bare prefix, a two-code tail or
# an empty format is the destination-format error.
show("plain_at", lambda: memoryview(b"AB").cast("@").format)
show("at_two", lambda: memoryview(b"AB").cast("@ii").format)
show("empty", lambda: memoryview(b"AB").cast("").format)
show("bad_prefix", lambda: memoryview(b"AB").cast(">i").format)
