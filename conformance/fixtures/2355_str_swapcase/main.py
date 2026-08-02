# str.swapcase lowercases the uppercase characters and uppercases the lowercase
# ones, leaving the titlecase and caseless characters alone. It applies the full
# Unicode mappings, which Go's simple 1:1 mapping only approximates: the German
# sharp s uppercases to SS, the ligatures expand, and the lowercased side applies
# the Final_Sigma rule, so a word-final uppercase sigma folds to the final form.
print("sharp s", "ß".swapcase())
print("mixed expand", "aBßc Σ".swapcase())
print("ligature", "ﬃ X".swapcase())
print("digraph up", "džungla".swapcase())
print("titlecase kept", "ǅ".swapcase())
print("word final sigma", "ΟΔΟΣ".swapcase())
print("lower to upper sigma", "οδος".swapcase())
print("dotted I", "İ".swapcase(), [hex(ord(c)) for c in "İ".swapcase()])
print("micro sign", "µ".swapcase())

# A standalone uppercase sigma is not word-final (no preceding cased letter), so
# it uppercases-then-lowercases to the plain form, and a case-ignorable mark
# between a cased letter and the sigma leaves it word-final.
print("bare sigma", "Σ".swapcase())
print("mark skip", "ΟΣ̇".swapcase())

# Plain text and the empty string are unchanged in shape, and a character with no
# case (a digit, an emoji, a CJK ideograph) is returned as is.
for s in ["", "HeLLo World", "123", "\U0001f600", "中文"]:
    print(repr(s), repr(s.swapcase()))

# swapcase takes no arguments, and it is a bound method that reads back off the
# instance.
try:
    "x".swapcase(1)
except TypeError as e:
    print("arity", e)
print("callable", callable("x".swapcase))

# A lone surrogate has no case and is returned unchanged, on its own and mixed
# into text, where it does not count as a following cased letter for the sigma.
low = b"\xed\xb2\x80".decode("utf-8", "surrogatepass")
print("surrogate", repr(low.swapcase()))
print("surrogate mixed", repr(("ΟΣ" + low).swapcase()))

# swapcase applied twice restores the plain letters where the mapping is 1:1, and
# stays stable on the expanding cases (swapcasing SS back gives ss, not the sharp
# s, which is expected).
for s in ["HeLLo", "ΟΔΟΣ", "abc"]:
    print("double", repr(s), s.swapcase().swapcase())
