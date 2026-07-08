# memoryview.cast reinterprets the underlying bytes under a new native format,
# and the reads that follow decode whole elements, not single bytes.

# Little-endian bytes for the 32-bit values 1, 258, 65535, 16909060.
b = b"\x01\x00\x00\x00\x02\x01\x00\x00\xff\xff\x00\x00\x04\x03\x02\x01"
mv = memoryview(b)
print(mv.format, mv.itemsize, mv.nbytes, len(mv))

u = mv.cast("I")
print(u.format, u.itemsize, len(u))
print(u.tolist())
print(u[0], u[3])
print(258 in u, 999 in u)
print(list(u))

# A signed cast sign-extends each element.
s = memoryview(b"\xff\xfe\x00\x01").cast("b")
print(s.tolist())
h = memoryview(b"\xff\xff\x00\x01").cast("h")
print(h.tolist())

# Casting back to bytes recovers the original element size.
back = u.cast("B")
print(back.format, back.itemsize, len(back))
print(bytes(back) == b)

# A length that is not a multiple of the target itemsize is rejected.
try:
    memoryview(b"\x00\x01\x02").cast("I")
except TypeError as e:
    print("badlen:", e)

# Casting between two non-byte formats is not supported.
try:
    u.cast("H")
except TypeError as e:
    print("nonbyte:", e)
