import struct


def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# A p ("Pascal string") field wider than 256 bytes keeps up to count-1 data
# bytes and caps only its recorded length byte at 255, so the data past byte 255
# survives rather than being truncated and zero-padded to the field width. The
# unpack side reads back the 255 bytes the length byte names.
for code, n in [("1000p", 1000), ("300p", 400), ("256p", 300), ("257p", 500)]:
    data = bytes([65 + (i % 3) for i in range(n)])
    show("pack " + code, (lambda c, d: (lambda: struct.pack(c, d).hex()))(code, data))
    show("roundtrip " + code, (lambda c, d: (lambda: struct.unpack(c, struct.pack(c, d))[0]))(code, data))

# The small cases still behave: the length byte is the copied length and the
# field is zero-padded past the data.
for code, data in [("0p", b"abc"), ("p", b"abc"), ("2p", b"abc"), ("5p", b"abc"), ("6p", b"abc")]:
    show("pack " + code, (lambda c, d: (lambda: struct.pack(c, d).hex()))(code, data))
    show("roundtrip " + code, (lambda c, d: (lambda: struct.unpack(c, struct.pack(c, d))[0]))(code, data))
