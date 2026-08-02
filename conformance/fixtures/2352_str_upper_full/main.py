# str.upper applies the full Unicode uppercase, which Go's simple 1:1 mapping
# only approximates: the German sharp s expands to SS, the ligatures expand to
# their letters, and the combining Greek ypogegrammeni uppercases to a capital
# iota. Uppercasing applies no context rule, so each code point maps on its own.
print("sharp s", "ß".upper())
print("strasse", "straße".upper())
print("ligature fi", "ﬁ".upper())
print("ligature ffi", "ﬃ".upper())
print("micro sign", "µ".upper())
print("digraph dz", "ǆ".upper())
print("ypogegrammeni", "ͅ".upper())
print("sigma", "ΟΔΟΣ".upper())

# Plain text and the ordinary uppercase path are unchanged, the empty string
# uppercases to itself, and a character with no uppercase (a digit, an emoji, a
# CJK ideograph) is returned as is.
for s in ["", "HeLLo World", "ALREADY UPPER", "123", "\U0001f600", "中文"]:
    print(repr(s), repr(s.upper()))

# A code point in a Unicode block newer than Go's own tables still uppercases
# correctly through the pinned map, checked by code point so the result does not
# depend on repr knowing the newer block.
c = chr(0x10D70)
print("new block", hex(ord(c.upper())), len(c.upper()))

# upper takes no arguments, and a lone surrogate has no uppercase and passes
# through unchanged, on its own and mixed into text that does map.
try:
    "x".upper(1)
except TypeError as e:
    print("arity", e)
low = b"\xed\xb2\x80".decode("utf-8", "surrogatepass")
print("surrogate", repr(low.upper()))
print("surrogate mixed", repr(("aß" + low).upper()))

# The expansions round trip in length the way CPython reports: the sharp s
# uppercases to two characters.
print("len expand", len("ß".upper()), len("ﬃ".upper()))
