import struct


def show(fmt, value):
    label = fmt + " " + repr(value)
    try:
        print(label, "->", struct.pack(fmt, value).hex())
    except Exception as e:
        print(label, "-> ERR", type(e).__name__, ":", e)


# Finite values at and below the format maximum pack cleanly, including the
# largest representable half and single and an explicit infinity.
show(">e", 65504.0)
show(">e", 65519.0)
show(">e", float("inf"))
show(">e", float("-inf"))
show(">e", 1.5)
show(">e", True)
show(">f", 3.4e38)
show(">f", float("inf"))
show(">d", float("inf"))
show(">d", 1e300)

# A finite float that overflows the e or f format raises OverflowError naming
# the format, and packing a negative overflow raises the same way.
show(">e", 65520.0)
show(">e", 70000.0)
show(">e", -70000.0)
show(">f", 1e40)
show(">f", -1e40)

# An int argument that converts to a finite double but then overflows the format
# raises struct.error "int too large to convert".
show(">e", 70000)
show(">e", -70000)
show(">f", 10 ** 40)

# An int too large for a double at all fails the conversion first and raises
# struct.error "required argument is not a float", the same message a non-number
# argument raises.
show(">e", 10 ** 400)
show(">f", 10 ** 400)
show(">d", 10 ** 400)
show(">e", "x")

# The same behavior holds through a compiled Struct instance and through the
# little-endian orders.
s = struct.Struct("<e")
try:
    print("Struct('<e').pack(70000.0) ->", s.pack(70000.0).hex())
except Exception as e:
    print("Struct('<e').pack(70000.0) -> ERR", type(e).__name__, ":", e)
show("<e", 70000)
show("<f", 1e40)
