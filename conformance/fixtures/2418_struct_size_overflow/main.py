# A struct format's total byte size is bounded by the platform ssize_t. A repeat
# count large enough that count times the item size (or the running total) would
# pass that ceiling is a struct.error, "total struct size too long", not a
# silently wrapped negative or garbage size. unagi used to accumulate the size in
# a machine int with no overflow guard, so a huge count wrapped around instead of
# raising. This pins the overflow boundary, that the same error surfaces from
# calcsize, Struct, pack and unpack, and that ordinary formats are untouched.
import struct

# Counts that overflow the running size raise, on several element sizes.
for fmt in (
    "1000000000000000000000h",
    "9223372036854775807h",
    "9223372036854775808b",
    "1152921504606846976q",
    "4611686018427387904d",
    "99999999999999999999999999999999999999h",
):
    try:
        struct.calcsize(fmt)
        print(repr(fmt), "no error")
    except struct.error as e:
        print(repr(fmt), "struct.error:", e)

# The maximum in-range count on a one-byte code is exactly ssize_t max and sizes
# without error, its size is that count.
maxcount = "9223372036854775807b"
print("maxcount size:", struct.calcsize(maxcount))

# The same overflow surfaces from every entry point, not just calcsize.
big = "9999999999999999999d"
for op in ("calcsize", "Struct", "pack", "unpack"):
    try:
        if op == "calcsize":
            struct.calcsize(big)
        elif op == "Struct":
            struct.Struct(big)
        elif op == "pack":
            struct.pack(big, 1.0)
        else:
            struct.unpack(big, b"")
        print(op, "no error")
    except struct.error as e:
        print(op, "struct.error:", e)

# Ordinary formats still size correctly, including native alignment padding and a
# large but in-range pad run.
for fmt in (">iih", "10s", "3f2d", "@qibh", "=4i", "100x", "0i", "", "@ci", "@cd", "100000000x"):
    print(repr(fmt), struct.calcsize(fmt))

# A count with no following code is still the earlier, unrelated error.
try:
    struct.calcsize("123")
except struct.error as e:
    print("no code:", e)
