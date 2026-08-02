# str.casefold applies the full Unicode case folding, which str.lower only
# approximates: the German sharp s folds to ss so a caseless match sees Maße and
# MASSE as equal, the ligatures expand, the Greek and Cyrillic letters fold to
# their canonical form, and casefold does not apply the final-sigma rule, so a
# word-final capital sigma folds to the ordinary lowercase sigma rather than the
# final form.
print("sharp s", "ß".casefold())
print("cap sharp s", "ẞ".casefold())
print("caseless masse", "MASSE".casefold() == "Maße".casefold())
print("ligature ffi", "ﬃ".casefold())
print("micro sign", "µ".casefold(), "µ".casefold() == "μ".casefold())
print("final sigma casefold", "ΟΔΟΣ".casefold())
print("greek theta", "ϑ".casefold())
print("cyrillic", "ᲀ".casefold(), "В".casefold())

# Plain text and the ordinary lowercase path are unchanged, the empty string
# folds to itself, and a character with no fold (a digit, an emoji, a CJK
# ideograph) is returned as is.
for s in ["", "HeLLo World", "already lower", "123", "\U0001f600", "中文"]:
    print(repr(s), repr(s.casefold()))

# casefold takes no arguments, and it is a bound method that reads back off the
# instance.
try:
    "x".casefold(1)
except TypeError as e:
    print("arity", e)
print("callable", callable("x".casefold))

# A lone surrogate has no fold and is returned unchanged, on its own and mixed
# into text that does fold.
low = b"\xed\xb2\x80".decode("utf-8", "surrogatepass")
print("surrogate", repr(low.casefold()))
print("surrogate mixed", repr(("Aß" + low).casefold()))

# casefold folds a longer run so idempotence holds: folding a folded string is a
# no-op across the expanding cases.
for s in ["ẞ", "MASSE", "ΣΊΣΥΦΟΣ", "ﬄ"]:
    f = s.casefold()
    print("idempotent", repr(s), f == f.casefold())
