# str.title and str.capitalize apply the full Unicode titlecase and lowercase,
# which Go's simple 1:1 mapping only approximates. title titlecases the first
# cased character of each word and lowercases the rest; capitalize titlecases the
# first character and lowercases the rest. The German sharp s titlecases to Ss and
# the ligatures expand, and the lowercased tail applies the Final_Sigma rule, so a
# word-final Greek capital sigma folds to the final form.
print("sharp s title", "ß test".title())
print("sharp s cap", "ß test".capitalize())
print("ligature title", "ﬃ ﬀ".title())
print("digraph title", "džungla".title())
print("titlecase char", "ǅ".title(), "ǅ".capitalize())
print("word final sigma", "ΟΔΟΣ".title())
print("two words sigma", "ΔΊΔΥΜΟΣ ΑΔΕΛΦΟΣ".title())
print("leading sigma", "ΣΊΣΥΦΟΣ".title())
print("cap sigma tail", "ΟΔΟΣ".capitalize())
print("dotted I title", "İstanbul".title(), [hex(ord(c)) for c in "İstanbul".title()])

# The word boundary is any uncased character, so an apostrophe or a digit breaks a
# word the way CPython's loop does, and a case-ignorable mark between the cased
# letter and a tail sigma leaves it word-final.
print("apostrophe", "it's a test".title())
print("digit", "3g ab".title())
print("mark skip", "ΟΣ̇ x".title())

# Plain text and the empty string are unchanged, and a character with no mapping
# (a digit, an emoji, a CJK ideograph) is returned as is.
for s in ["", "HELLO World", "123", "\U0001f600", "中文"]:
    print(repr(s), repr(s.title()), repr(s.capitalize()))

# title and capitalize take no arguments, and they are bound methods that read
# back off the instance.
try:
    "x".title(1)
except TypeError as e:
    print("title arity", e)
try:
    "x".capitalize(1)
except TypeError as e:
    print("capitalize arity", e)
print("callable", callable("x".title), callable("x".capitalize))

# A lone surrogate has no mapping and is returned unchanged, on its own and mixed
# into text, where it does not count as a cased letter for the word boundary.
low = b"\xed\xb2\x80".decode("utf-8", "surrogatepass")
print("surrogate", repr(low.title()), repr(low.capitalize()))
print("surrogate mixed", repr(("ab" + low + "cd").title()))

# title is idempotent across the expanding and context cases.
for s in ["ß test", "ΟΔΟΣ", "ΣΊΣΥΦΟΣ", "ﬃ x"]:
    f = s.title()
    print("idempotent", repr(s), f == f.title())
