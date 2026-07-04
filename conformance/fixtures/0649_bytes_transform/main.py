# bytes and bytearray transform and predicate methods, ASCII semantics.

# Case transforms.
print(b"Hello, World! 123".upper())
print(b"Hello, World! 123".lower())
print(b"HeLLo".swapcase())
print(b"hELLO world".capitalize())
print(b"they're bill's friends".title())
print(b"\xe9abc".upper())

# Strip family.
print(b"  x y  ".strip())
print(b"  x y  ".lstrip())
print(b"  x y  ".rstrip())
print(b"xxabcxx".strip(b"x"))
print(b"abc".strip(None))

# Replace.
print(b"aaa".replace(b"a", b"bb"))
print(b"aaa".replace(b"a", b"bb", 2))
print(b"abc".replace(b"", b"-"))
print(b"abc".replace(b"", b"-", 2))

# Prefix and suffix removal.
print(b"hello".removeprefix(b"he"))
print(b"hello".removeprefix(b"xy"))
print(b"hello".removesuffix(b"lo"))
print(b"hello".removesuffix(b"xy"))

# Padding.
print(b"hello".center(11, b"*"))
print(b"hello".center(10, b"*"))
print(b"hi".center(4))
print(b"hi".ljust(4))
print(b"hi".rjust(4))
print(b"-42".zfill(5))
print(b"+7".zfill(5))
print(b"".zfill(3))
print(b"abc".zfill(2))

# Predicates.
print(b"".isascii(), b"\x80".isascii())
print(b"".isalpha(), b"abc".isalpha())
print(b"abc1".isalnum(), b"ab c".isalnum())
print(b"123".isdigit(), b"12x".isdigit())
print(b"  \t\n".isspace(), b"a".isspace())
print(b"abc1".islower(), b"123".islower(), b"Abc".islower())
print(b"ABC1".isupper(), b"abc".isupper())
print(b"Hello World".istitle(), b"hello".istitle())

# bytearray returns bytearray from every transform.
ba = bytearray(b"AbC")
print(ba.lower())
print(bytearray(b"  x  ").strip())
print(bytearray(b"a-b").replace(b"-", b"+"))
print(bytearray(b"hi").center(6, b"."))
print(bytearray(b"ab").isalpha())

# Error catalog.
try:
    b"ab".strip("x")
except TypeError as e:
    print("stripstr", e)
try:
    b"ab".replace("a", b"b")
except TypeError as e:
    print("replaceold", e)
try:
    b"ab".replace(b"a", b"b", "2")
except TypeError as e:
    print("replacecount", e)
try:
    b"ab".ljust("4")
except TypeError as e:
    print("ljustwidth", e)
try:
    b"ab".ljust(4, b"xy")
except TypeError as e:
    print("ljustfilllen", e)
try:
    b"ab".ljust(4, "x")
except TypeError as e:
    print("ljustfillstr", e)
