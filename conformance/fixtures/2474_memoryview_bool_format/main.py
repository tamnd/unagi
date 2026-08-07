def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# Casting a bytes view to the '?' code reads each byte back as a bool, with any
# non-zero byte being True.
src = bytes([0, 1, 2, 255, 0])
mv = memoryview(src).cast("?")
show("format", lambda: mv.format)
show("itemsize", lambda: mv.itemsize)
show("nbytes", lambda: mv.nbytes)
show("len", lambda: len(mv))
show("tolist", lambda: mv.tolist())
show("index_0", lambda: mv[0])
show("index_1", lambda: mv[1])
show("index_3", lambda: mv[3])
show("elem_type", lambda: type(mv[1]).__name__)
show("contains_true", lambda: True in mv)
show("contains_false", lambda: False in mv)

# A store through a '?' view records the truthiness of any object as a single
# 0 or 1 byte, the way struct '?' pack runs it through bool().
buf = bytearray(6)
w = memoryview(buf).cast("?")
show("write_int_nonzero", lambda: (w.__setitem__(0, 5), w[0])[1])
show("write_true", lambda: (w.__setitem__(1, True), w[1])[1])
show("write_false", lambda: (w.__setitem__(2, False), w[2])[1])
show("write_float", lambda: (w.__setitem__(3, 0.0), w[3])[1])
show("write_str", lambda: (w.__setitem__(4, "x"), w[4])[1])
show("write_none", lambda: (w.__setitem__(5, None), w[5])[1])
show("buf_after_writes", lambda: bytes(buf))
show("tobytes", lambda: w.tobytes())

# The written view round-trips through tolist and equals a matching bytes view.
show("tolist_after", lambda: w.tolist())
show("eq_bytes_view", lambda: w == memoryview(bytes([1, 1, 0, 0, 1, 0])).cast("?"))

# A '?' view is one byte per element, so a three byte buffer holds three bools.
show("three_bytes", lambda: memoryview(bytes([0, 7, 0])).cast("?").tolist())
