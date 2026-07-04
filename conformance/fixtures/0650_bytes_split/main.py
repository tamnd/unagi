# bytes and bytearray split, join, partition and translate methods.

# split on whitespace and on a separator.
print(b"a b  c".split())
print(b"  a  b  c  ".split())
print(b"a,b,,c".split(b","))
print(b"a,b,c".split(b",", 1))
print(b"  a  b  c  ".split(None, 1))
print(b"".split())
print(b"".split(b","))

# rsplit.
print(b"a,b,c".rsplit(b",", 1))
print(b"  a  b  c  ".rsplit(None, 1))
print(b"a,b,c".rsplit(b",", 5))

# splitlines: bytes splits only on \n, \r and \r\n.
print(b"a\nb\r\nc\rd".splitlines())
print(b"a\nb\n".splitlines())
print(b"a\nb\n".splitlines(True))
print(b"\r\n\n".splitlines())
print(b"\x0b\x0c\x1c".splitlines())

# join, returning the separator's type.
print(b",".join([b"a", b"b", b"c"]))
print(b"-".join([bytearray(b"x"), b"y"]))
print(b",".join([]))

# partition and rpartition.
print(b"AxBxC".partition(b"x"))
print(b"AxBxC".rpartition(b"x"))
print(b"ABC".partition(b"x"))
print(b"ABC".rpartition(b"x"))

# translate: build a table with a mutable bytearray, then map through it.
table = bytearray(range(256))
table[ord("a")] = ord("A")
table[ord("b")] = ord("B")
print(b"abcabc".translate(bytes(table)))
print(b"abcabc".translate(None, b"b"))
print(b"abcabc".translate(bytes(table), b"c"))
print(b"abc".translate(None))

# bytearray returns bytearray pieces.
print(bytearray(b"a,b,c").split(b","))
print(bytearray(b"AxB").partition(b"x"))
print(bytearray(b"-").join([b"p", b"q"]))

# Error catalog.
try:
    b"a".split(b"")
except ValueError as e:
    print("splitempty", e)
try:
    b"a b".split(1)
except TypeError as e:
    print("splitint", e)
try:
    b"a".split(b",", "x")
except TypeError as e:
    print("splitmax", e)
try:
    b"a".partition(b"")
except ValueError as e:
    print("partempty", e)
try:
    b"a".partition("x")
except TypeError as e:
    print("partstr", e)
try:
    b",".join([b"a", "b"])
except TypeError as e:
    print("joinstr", e)
try:
    b",".join(5)
except TypeError as e:
    print("joinnoniter", e)
try:
    b"abc".translate(b"short")
except ValueError as e:
    print("transshort", e)
