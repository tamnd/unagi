# float() accepts a signed nan string and keeps the sign bit, and the quiet nan
# unagi builds for float("nan"), math.nan and float.fromhex matches CPython's
# canonical 0x7ff8...0000 payload so struct.pack and float.hex are byte-identical.
# Only the constructor and constant nan sources are checked: a nan produced by
# arithmetic (inf - inf) carries a platform-dependent sign and is out of scope.
import math
import struct
import cmath


def bits(f):
    return struct.pack(">d", f).hex()


def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as ex:
        print(label, type(ex).__name__, str(ex))


# float() parsing of nan and inf variants.
for s in ["nan", "NaN", "NAN", "-nan", "+nan", "-NaN", "  -nan  ", "nan ",
          "inf", "-inf", "+inf", "Infinity", "-Infinity", "infinity",
          "-nanx", "nana", "-", "+"]:
    show("float(%r)" % s, lambda s=s: float(s))

# Sign bit and payload of the various nan sources.
sources = {
    "float-nan": float("nan"),
    "float-neg-nan": float("-nan"),
    "float-plus-nan": float("+nan"),
    "math-nan": math.nan,
    "math-neg-nan": -math.nan,
    "cmath-nan": cmath.nan,
    "cmath-nanj-imag": cmath.nanj.imag,
    "fromhex-nan": float.fromhex("nan"),
    "complex-nan-real": complex("nan").real,
    "abs-neg-nan": abs(float("-nan")),
}
for k in sorted(sources):
    v = sources[k]
    print(k, bits(v), math.copysign(1.0, v), v.hex())

print("struct-neg-nan-be", struct.pack(">d", float("-nan")).hex())
print("struct-neg-nan-le", struct.pack("<d", float("-nan")).hex())
print("isnan-neg", math.isnan(float("-nan")))
