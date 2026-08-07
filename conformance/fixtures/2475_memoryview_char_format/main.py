def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# Casting a bytes view to the 'c' code reads each element back as a length-one
# bytes object, not the int the 'B' path gives.
src = bytes([65, 66, 0, 255])
mv = memoryview(src).cast("c")
show("format", lambda: mv.format)
show("itemsize", lambda: mv.itemsize)
show("nbytes", lambda: mv.nbytes)
show("len", lambda: len(mv))
show("tolist", lambda: mv.tolist())
show("index_0", lambda: mv[0])
show("index_3", lambda: mv[3])
show("elem_type", lambda: type(mv[0]).__name__)
show("contains_hit", lambda: b"A" in mv)
show("contains_miss", lambda: b"Z" in mv)
show("count", lambda: memoryview(b"ABA").cast("c").count(b"A"))
show("index", lambda: memoryview(b"ABA").cast("c").index(b"B"))

# A store through a 'c' view takes a length-one bytes object and writes its byte.
buf = bytearray(4)
w = memoryview(buf).cast("c")
show("write_bytes", lambda: (w.__setitem__(0, b"Z"), w[0])[1])
show("write_null", lambda: (w.__setitem__(1, b"\x00"), w[1])[1])
show("write_high", lambda: (w.__setitem__(2, b"\xff"), w[2])[1])
show("buf_after", lambda: bytes(buf))
show("tobytes", lambda: w.tobytes())

# The wrong type is the invalid-type error, a wrong length the invalid-value one.
show("write_int", lambda: w.__setitem__(3, 5))
show("write_two", lambda: w.__setitem__(3, b"QR"))
show("write_empty", lambda: w.__setitem__(3, b""))
show("write_str", lambda: w.__setitem__(3, "Q"))
show("write_bytearray", lambda: w.__setitem__(3, bytearray(b"Q")))
show("write_memoryview", lambda: w.__setitem__(3, memoryview(b"Q")))
show("write_none", lambda: w.__setitem__(3, None))

# Two 'c' views over the same bytes compare equal.
show("eq", lambda: memoryview(b"AB").cast("c") == memoryview(b"AB").cast("c"))
