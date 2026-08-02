# str.lower applies the full Unicode lowercase, which Go's simple 1:1 mapping only
# approximates: the Turkish dotted capital I expands to i plus a combining dot,
# the capital sharp s lowercases to the ordinary sharp s, and the one context
# rule, Final_Sigma, folds a word-final Greek capital sigma to the final form ς
# while a sigma anywhere else in the word folds to the plain form σ.
print("dotted I", "İ".lower(), [hex(ord(c)) for c in "İ".lower()])
print("cap sharp s", "ẞ".lower())
print("word final", "ΟΔΟΣ".lower())
print("two words", "ΔΊΔΥΜΟΣ ΑΔΕΛΦΟΣ".lower())
print("leading sigma", "ΣΊΣΥΦΟΣ".lower())
print("bare sigma", "Σ".lower())
print("sigma both ends", "ΣΟΣ".lower())

# The walk skips case-ignorable characters on either side of the sigma, so a
# combining mark between the cased letter and the sigma leaves it word-final,
# while an uncased separator like a space breaks the word.
print("skip mark", "ΟΣ̇".lower())
print("space breaks", "ΟΣ ΟΣ".lower())
print("apostrophe skip", "ΟΣ'".lower())

# Plain text and the ordinary lowercase path are unchanged, the empty string
# lowercases to itself, and a character with no mapping (a digit, an emoji, a CJK
# ideograph) is returned as is.
for s in ["", "HeLLo World", "already lower", "123", "\U0001f600", "中文"]:
    print(repr(s), repr(s.lower()))

# lower takes no arguments, and it is a bound method that reads back off the
# instance.
try:
    "x".lower(1)
except TypeError as e:
    print("arity", e)
print("callable", callable("x".lower))

# A lone surrogate has no mapping and is returned unchanged, on its own and mixed
# into text that does lower, including right after a sigma where it must not count
# as a following cased letter.
low = b"\xed\xb2\x80".decode("utf-8", "surrogatepass")
print("surrogate", repr(low.lower()))
print("surrogate mixed", repr(("ΟΣ" + low).lower()))

# lower is idempotent: lowercasing a lowercased string is a no-op across the
# expanding and context cases.
for s in ["İ", "ẞ", "ΟΔΟΣ", "ΣΊΣΥΦΟΣ"]:
    f = s.lower()
    print("idempotent", repr(s), f == f.lower())
