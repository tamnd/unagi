def show(label, e):
    try:
        print(label, repr(e()))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, ex)


# A memoryview.cast view over a bytearray writes a whole typed element back, the
# value encoded little-endian under the destination format so a read-back
# decodes what was stored. Each write lands in the first element and leaves the
# rest of the buffer alone.
def w(fmt, val):
    def go():
        v = memoryview(bytearray(16)).cast(fmt)
        v[0] = val
        return v.tolist()[:2]

    return go


show("i", w("i", 999))
show("h-neg", w("h", -5))
show("q", w("q", 2**40))
show("Q-big", w("Q", 2**63 + 5))
show("i-neg", w("i", -(2**31)))
show("f", w("f", 1.5))
show("d", w("d", 1.5))
show("b-neg", w("b", -1))
show("B", w("B", 200))
show("bool", w("i", True))

# The range and type of the value are checked against the destination format
# with memoryview's own format-named messages: a value past the format's range
# is a ValueError, a value of the wrong type a TypeError.
show("H-over", w("H", 70000))
show("i-over", w("i", 2**31))
show("b-over", w("b", 200))
show("B-over", w("B", 300))
show("i-float", w("i", 1.5))
show("i-str", w("i", "x"))
show("f-str", w("f", "x"))

# Casting to a float format re-reads the bytes under that width, reporting the
# format and itemsize, and reads and writes round-trip through single and double
# precision.
fv = memoryview(bytes(8)).cast("f")
print("f-meta", fv.format, fv.itemsize, fv.ndim, fv.tolist())
dv = memoryview(bytes(8)).cast("d")
print("d-meta", dv.format, dv.itemsize, dv.ndim, dv.tolist())


# A single-precision write rounds to float32, so a value with no exact single
# representation reads back as its rounded single.
def fround():
    v = memoryview(bytearray(4)).cast("f")
    v[0] = 0.1
    return v[0]


show("f-round", fround)

# Casting between two non-byte formats is still rejected, so a float view cannot
# be recast to another wide format without going through bytes first.
show("f-to-i", lambda: memoryview(bytearray(8)).cast("f").cast("i"))

# A typed write addresses through a multi-dimensional cast too: the tuple key
# resolves to one element and the value is encoded under the shared format.
md = memoryview(bytearray(24)).cast("i", shape=[2, 3])
md[1, 2] = 12345
md[0, 0] = -7
print("md", md.tolist())

# A float view reshaped to two dimensions writes and reads the same way.
mf = memoryview(bytearray(24)).cast("d", shape=[3])
mf[1] = 2.5
print("mf", mf.tolist())
