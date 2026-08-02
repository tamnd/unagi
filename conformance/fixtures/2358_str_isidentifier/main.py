# str.isidentifier is true for a non-empty string whose first character starts an
# identifier (the XID_Start property, plus the underscore) and whose remaining
# characters continue one (XID_Continue). It reads from the pinned CPython 3.14.6
# XID tables, which cover the newer blocks Go's older unicode data misses: the
# Garay letters start and continue identifiers even though older tables do not
# know them.
print("plain", "name".isidentifier())
print("underscore", "_hidden".isidentifier())
print("digits", "a1b2".isidentifier())
print("leading digit", "1abc".isidentifier())
print("dunder", "__init__".isidentifier())
print("empty", "".isidentifier())
print("space", "a b".isidentifier())
print("dash", "a-b".isidentifier())
print("dot", "a.b".isidentifier())
print("keyword", "class".isidentifier())

# Non-ASCII identifiers: a Greek letter, an accented letter and a combining mark
# after a starter all continue an identifier; a lone combining mark cannot start
# one.
print("greek", "πλ".isidentifier())
print("accented", "café".isidentifier())
print("combining", ("a" + "́").isidentifier())
print("combining start", ("́" + "a").isidentifier())
print("superscript", "x²".isidentifier())

# The Garay block (Unicode 16.0.0) is a set of letters that start and continue
# identifiers, so a name built from them is an identifier.
print("garay", "\U00010d50\U00010d70".isidentifier())
print("garay tail", ("v" + "\U00010d70").isidentifier())

# A no-break space and a zero-width space do not continue an identifier, but a
# Roman numeral (a number letter, so part of XID_Continue) does.
print("nbsp", "a\xa0b".isidentifier())
print("zwsp", ("a" + "​").isidentifier())
print("roman", ("x" + "Ⅰ").isidentifier())

# isidentifier takes no arguments and is a bound method off the instance.
try:
    "x".isidentifier(1)
except TypeError as e:
    print("arity", e)
print("callable", callable("x".isidentifier))

# A lone surrogate does not start or continue an identifier, on its own and mixed
# into text.
low = b"\xed\xb2\x80".decode("utf-8", "surrogatepass")
print("surrogate", low.isidentifier())
print("surrogate tail", ("a" + low).isidentifier())
