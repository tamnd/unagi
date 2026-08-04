import warnings
warnings.filterwarnings("ignore", category=DeprecationWarning)
import array
import struct

reconstruct = array._array_reconstructor

# The machine format codes as CPython's array module numbers them.
UNSIGNED_INT8, SIGNED_INT8 = 0, 1
UNSIGNED_INT16_LE, UNSIGNED_INT16_BE, SIGNED_INT16_LE, SIGNED_INT16_BE = 2, 3, 4, 5
UNSIGNED_INT32_LE, UNSIGNED_INT32_BE, SIGNED_INT32_LE, SIGNED_INT32_BE = 6, 7, 8, 9
UNSIGNED_INT64_LE, UNSIGNED_INT64_BE, SIGNED_INT64_LE, SIGNED_INT64_BE = 10, 11, 12, 13
IEEE_754_FLOAT_LE, IEEE_754_FLOAT_BE, IEEE_754_DOUBLE_LE, IEEE_754_DOUBLE_BE = 14, 15, 16, 17
UTF16_LE, UTF16_BE, UTF32_LE, UTF32_BE = 18, 19, 20, 21

# The fast path: the recorded machine format matches this platform, so the raw
# bytes read straight through.
print(reconstruct(array.array, "b", SIGNED_INT8, b"\xff\x02"))
print(reconstruct(array.array, "B", UNSIGNED_INT8, b"\x80\x7f\x00\xff"))
print(reconstruct(array.array, "h", SIGNED_INT16_LE, struct.pack("<hh", -0x8000, 0x7fff)))
print(reconstruct(array.array, "d", IEEE_754_DOUBLE_LE, struct.pack("<dd", 1.5, -2.0)))

# The slow path: a different word size or byte order, so the bytes decode under
# the recorded format and the integer codes retype to the matching width.
retype = (
    (["H", "I", "L"], UNSIGNED_INT16_BE, ">HHH", [0x8000, 0x7fff, 0xffff]),
    (["h", "i", "l"], SIGNED_INT16_BE, ">hhh", [-0x8000, 0x7fff, 0]),
    (["I", "L"], UNSIGNED_INT32_BE, ">II", [1 << 31, (1 << 32) - 1]),
    (["i", "l"], SIGNED_INT32_BE, ">ii", [-1 << 31, (1 << 31) - 1]),
    (["L"], UNSIGNED_INT64_BE, ">QQ", [1 << 63, (1 << 64) - 1]),
    (["l"], SIGNED_INT64_BE, ">qq", [-1 << 63, (1 << 63) - 1]),
    (["f"], IEEE_754_FLOAT_BE, ">ff", [1.5, -2.0]),
    (["d"], IEEE_754_DOUBLE_BE, ">dd", [1.5, -2.0]),
)
for codes, mf, sfmt, vals in retype:
    packed = struct.pack(sfmt, *vals)
    for code in codes:
        got = reconstruct(array.array, code, mf, packed)
        print(code, mf, got.typecode, got.tolist())

# The unicode codes decode UTF-16 or UTF-32 back to text under the byte order.
text16 = "Bonjour\xe9"
text32 = "AB\U0002030a\U00020347"
cp16 = [ord(c) for c in text16]
cp32 = [ord(c) for c in text32]
for code in "uw":
    for mf, sfmt, cps, want in (
        (UTF16_LE, "<8H", cp16, text16),
        (UTF16_BE, ">8H", cp16, text16),
        (UTF32_LE, "<4I", cp32, text32),
        (UTF32_BE, ">4I", cp32, text32),
    ):
        got = reconstruct(array.array, code, mf, struct.pack(sfmt, *cps))
        print(code, mf, got.typecode, got.tounicode() == want)

# The error surface, matched message for message with CPython.
def show(*a):
    try:
        reconstruct(*a)
        print("no error")
    except Exception as ex:
        print(type(ex).__name__ + ":", ex)

show("", "b", 0, b"")
show(str, "b", 0, b"")
show(array.array, "b", "", b"")
show(array.array, "b", 0, "")
show(array.array, "?", 0, b"")
show(array.array, "b", -1, b"")
show(array.array, "b", 22, b"")
show(array.array, "d", IEEE_754_DOUBLE_BE, b"short")
show(array.array, "bb", 0, b"")
