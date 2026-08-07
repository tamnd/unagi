import struct


def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# The C99 complex codes 'F' (two float32) and 'D' (two float64) that CPython
# 3.14 grew: a complex float is 8 bytes, a complex double 16, and native
# alignment pads a leading byte up to the component width.
for fmt in ["F", "D", "2F", "@bF", "@bD", "<F", ">D", "=D"]:
    show("size " + fmt, (lambda f: (lambda: struct.calcsize(f)))(fmt))

# pack writes the real component then the imaginary in the format's byte order,
# and unpack reads them back to a complex.
for fmt in ["F", "D", "<F", ">F", "<D", ">D"]:
    for val in [3 + 4j, -1.5 + 0.25j, 0j, complex(0.0, -0.0)]:
        show("pack %s %r" % (fmt, val),
             (lambda f, v: (lambda: struct.pack(f, v).hex()))(fmt, val))
        show("roundtrip %s %r" % (fmt, val),
             (lambda f, v: (lambda: struct.unpack(f, struct.pack(f, v))))(fmt, val))

# A complex, float, int or bool all coerce; any other type is a struct.error.
show("int", lambda: struct.pack("F", 5).hex())
show("float", lambda: struct.pack("D", 2.5).hex())
show("bool", lambda: struct.pack("F", True).hex())
show("str", lambda: struct.pack("F", "x"))
show("list", lambda: struct.pack("D", [1]))

# Unlike the scalar float codes, an overflowing component rounds to infinity
# with no error.
show("ovf F", lambda: struct.pack("F", 1e300 + 0j).hex())
show("ovf D", lambda: struct.pack("D", 1e400 + 0j).hex())

# A complex mixed with other fields packs and unpacks in place.
show("mixed pack", lambda: struct.pack("@bF", 1, 2 + 3j).hex())
show("mixed unpack", lambda: struct.unpack("@bF", struct.pack("@bF", 1, 2 + 3j)))
