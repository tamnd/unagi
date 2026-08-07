def w(code, val):
    b = bytearray(8)
    m = memoryview(b).cast(code)
    try:
        m[0] = val
        return bytes(b).hex()
    except Exception as e:
        return "ERR " + type(e).__name__ + " " + str(e)


def rt(code, val):
    b = bytearray(8)
    m = memoryview(b).cast(code)
    m[0] = val
    return m[0]


def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, str(ex))


buf8 = b"\x01\x02\x03\x04\x05\x06\x07\x08"

# memoryview.cast now accepts the native-width integer codes, all eight bytes on
# this 64-bit target: n and l are signed like q, N and L unsigned like Q, and P
# is a pointer that takes either the signed or the unsigned 64-bit range. The
# view reports the code it was cast to while reading and writing the underlying
# eight-byte little-endian value.
for code in ["n", "N", "l", "L", "P"]:
    show("read-" + code, (lambda c: (lambda: list(memoryview(buf8).cast(c))))(code))
    show("itemsize-" + code, (lambda c: (lambda: memoryview(buf8).cast(c).itemsize))(code))
    show("format-" + code, (lambda c: (lambda: memoryview(buf8).cast(c).format))(code))

# Signed codes read a negative value back, the unsigned codes a full-width value.
neg8 = (-1).to_bytes(8, "little", signed=True)
show("neg-n", lambda: list(memoryview(neg8).cast("n")))
show("neg-l", lambda: list(memoryview(neg8).cast("l")))
show("big-N", lambda: list(memoryview((2**64 - 1).to_bytes(8, "little")).cast("N")))
show("big-L", lambda: list(memoryview((2**64 - 1).to_bytes(8, "little")).cast("L")))

# Writes: the signed codes wrap a negative value two's-complement and reject an
# out-of-range one, the unsigned codes reject a negative one, and P uniquely
# accepts both a negative value (wrapping it) and the full unsigned range.
show("write-n-neg", lambda: w("n", -5))
show("write-n-over", lambda: w("n", 2**63))
show("write-N-neg", lambda: w("N", -1))
show("write-N-max", lambda: w("N", 2**64 - 1))
show("write-N-over", lambda: w("N", 2**64))
show("write-l-neg", lambda: w("l", -5))
show("write-L-max", lambda: w("L", 2**64 - 1))
show("write-P-neg", lambda: w("P", -1))
show("write-P-max", lambda: w("P", 2**64 - 1))
show("write-P-over", lambda: w("P", 2**64))

# Round trips through the read path and a re-cast back to bytes.
show("rt-n", lambda: rt("n", -12345))
show("rt-N", lambda: rt("N", 2**64 - 1))
show("rt-P", lambda: rt("P", 2**63))
show("multi-l", lambda: list(memoryview(bytes(range(16))).cast("l")))
show("cast-back", lambda: bytes(memoryview(buf8).cast("n").cast("B")))

# A buffer whose length is not a multiple of the native itemsize still raises the
# cast size error, and a non-integer store the invalid-type TypeError.
show("wrong-size", lambda: memoryview(b"\x01\x02\x03").cast("n"))
show("bad-type", lambda: w("l", 1.5))
