import struct


def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# Casting a bytes view to the 'e' code reads each two-byte element back as a
# float, decoding the IEEE 754 half the way struct 'e' unpack does.
src = struct.pack("<4e", 1.0, -2.5, 0.0, 65504.0)
mv = memoryview(src).cast("e")
show("format", lambda: mv.format)
show("itemsize", lambda: mv.itemsize)
show("nbytes", lambda: mv.nbytes)
show("len", lambda: len(mv))
show("tolist", lambda: mv.tolist())
show("index_0", lambda: mv[0])
show("index_3", lambda: mv[3])
show("elem_type", lambda: type(mv[0]).__name__)

# A store through an 'e' view rounds the number to half precision.
buf = bytearray(2 * 8)
w = memoryview(buf).cast("e")
show("write_exact", lambda: (w.__setitem__(0, 1.5), w[0])[1])
show("write_int", lambda: (w.__setitem__(1, 3), w[1])[1])
show("write_bool", lambda: (w.__setitem__(2, True), w[2])[1])
show("write_round", lambda: (w.__setitem__(3, 1.0 / 3.0), w[3])[1])
show("write_max", lambda: (w.__setitem__(4, 65504.0), w[4])[1])
show("write_subnormal", lambda: (w.__setitem__(5, 1e-5), w[5])[1])
show("write_negzero", lambda: (w.__setitem__(6, -0.0), struct.pack("<e", w[6]))[1])
show("tobytes", lambda: w.tobytes())

# A real infinity or nan passes through the half format.
show("write_inf", lambda: (w.__setitem__(7, float("inf")), w[7])[1])
show("write_ninf", lambda: (w.__setitem__(7, float("-inf")), w[7])[1])
show("write_nan", lambda: (w.__setitem__(7, float("nan")), w[7])[1])

# A finite value past the half range is the invalid-value error, either sign.
show("overflow_pos", lambda: w.__setitem__(7, 1e5))
show("overflow_neg", lambda: w.__setitem__(7, -70000.0))

# A non-number is the invalid-type error.
show("write_str", lambda: w.__setitem__(7, "x"))
show("write_none", lambda: w.__setitem__(7, None))
show("write_bytes", lambda: w.__setitem__(7, b"a"))

# Two 'e' views over the same halves compare equal.
show("eq", lambda: memoryview(struct.pack("<2e", 1.5, 2.5)).cast("e")
     == memoryview(struct.pack("<2e", 1.5, 2.5)).cast("e"))
