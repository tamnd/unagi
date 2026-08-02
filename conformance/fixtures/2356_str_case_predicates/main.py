# str.isupper, str.islower and str.istitle classify each character with the
# Uppercase, Lowercase and titlecase (category Lt) properties. They read from the
# pinned CPython tables, which cover the newer bicameral blocks Go's older
# unicode data misses: the Garay letters and the double-struck mathematical
# capitals count as cased where Go would call them uncased.
print("upper plain", "HELLO".isupper())
print("lower plain", "hello".islower())
print("upper mixed", "Hello".isupper(), "Hello".islower())
print("upper with digits", "AB1".isupper(), "ab1".islower())
print("upper uncased only", "123".isupper(), "123".islower())
print("empty", "".isupper(), "".islower(), "".istitle())

# The titlecase digraph is neither upper nor lower on its own but does make a
# one-character title, and a Lt letter after a cased letter breaks a title.
print("digraph", "ǅ".isupper(), "ǅ".islower(), "ǅ".istitle())
print("digraph in word", "Aǅ".istitle())

# The Garay block (Unicode 16.0.0) is bicameral, so its capital is uppercase and
# its small letter is lowercase even though older tables do not know it.
print("garay cap", "\U00010d50".isupper(), "\U00010d50".islower())
print("garay small", "\U00010d70".isupper(), "\U00010d70".islower())
print("garay word", "\U00010d50\U00010d70".istitle())
print("garay two caps", "\U00010d50\U00010d50".istitle())

# A caseless-cased letter keeps its property while mapping to itself: the
# double-struck capital C is uppercase, the feminine ordinal is lowercase.
print("math cap", "ℂ".isupper(), "ℂ".islower())
print("ordinal", "ª".isupper(), "ª".islower())

# istitle wants each cased run to start upper or titlecase and continue lower,
# with uncased characters (spaces, digits, apostrophes) breaking the run.
for s in ["Hello World", "Hello world", "HELLO", "Hello  World",
          "It'S", "It's", "A", "1A1", "1a", " ", "ǅungla", "Ǆungla"]:
    print("istitle", repr(s), s.istitle())

# The sharp s and the ligatures are lowercase, the Roman numerals are cased both
# ways by their case, and the dotless small letters stay lower.
print("sharp s", "ß".islower(), "ß".isupper())
print("ligature", "ﬁ".islower())
print("roman upper", "Ⅰ".isupper(), "Ⅰ".islower())
print("roman lower", "ⅰ".isupper(), "ⅰ".islower())

# The predicates take no arguments and are bound methods off the instance.
try:
    "x".isupper(1)
except TypeError as e:
    print("arity", e)
print("callable", callable("x".isupper), callable("x".istitle))

# A lone surrogate has no case, so it never makes a string upper, lower or title
# and it does not count as a cased character mixed into text.
low = b"\xed\xb2\x80".decode("utf-8", "surrogatepass")
print("surrogate", low.isupper(), low.islower(), low.istitle())
print("surrogate mixed", ("A" + low).isupper(), ("A" + low + "b").istitle())
