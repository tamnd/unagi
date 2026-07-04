# bytearray, the mutable byte buffer, plus the bytes() and bytearray()
# constructors. Every value below prints identically under CPython 3.14.
# Per-method atomicity under concurrent mutation is proven separately by the
# race-detector unit test in pkg/objects, since imports land in a later
# milestone.
b = bytearray(b"abc")
print(b)
print(repr(b))
print(str(b))
print(bytearray(), bytearray(3))

# Constructors: count, copy, iterable of ints, str with an encoding.
print(bytes(), bytes(3), bytes(b"hi"), bytes(bytearray(b"hi")))
print(bytes([65, 66, 67]), bytearray([65, 66, 67]))
print(bytes("é", "utf-8"), bytes("é", "latin-1"))
print(bytearray("abc", "ascii"))
print(list(bytes("é", "utf-8")), list(bytes("é", "latin-1")))

# Indexing yields ints; slicing yields a fresh bytearray.
print(bytearray(b"abcde")[0], bytearray(b"abcde")[-1])
print(bytearray(b"abcde")[1:4], bytearray(b"abcde")[::-1])
print([x for x in bytearray(b"abc")])
print(len(bytearray(b"hello")), len(bytearray()))

# Mutation methods observed through an alias.
m = bytearray(b"abc")
alias = m
m.append(100)
m.extend(b"ef")
m.extend([103, 104])
m.insert(0, 90)
print(alias)
print(m.pop(), m.pop(0))
m.remove(101)
print(m)
m.reverse()
print(m)
cp = m.copy()
m.append(33)
print(m, cp)
m.clear()
print(m)

# setitem, slice assignment and deletion.
s = bytearray(b"abcde")
s[0] = 65
print(s)
s[0:2] = b"xy"
s[2:2] = b"__"
print(s)
del s[0]
del s[0:2]
print(s)

# In-place operators. += takes a bytes-like, *= repeats.
p = bytearray(b"ab")
p += b"cd"
p *= 2
print(p)

# Cross-type equality, ordering and concatenation.
print(bytearray(b"abc") == b"abc", b"abc" == bytearray(b"abc"))
print(bytearray(b"abc") < b"abd", b"a" < bytearray(b"b"))
print((bytearray(b"ab") + b"cd"), (b"ab" + bytearray(b"cd")))
print(bytearray(b"at") in bytearray(b"cat"), 97 in bytearray(b"cat"))

# Truthiness follows length.
print(bool(bytearray()), bool(bytearray(b"x")))

# Catchable errors, one per labelled line.
try:
    bytes([256])
except ValueError as e:
    print("bytes256", e)
try:
    bytearray([256])
except ValueError as e:
    print("bytearray256", e)
try:
    bytes(-1)
except ValueError as e:
    print("negcount", e)
try:
    bytes(1.5)
except TypeError as e:
    print("float", e)
try:
    bytes("abc")
except TypeError as e:
    print("noenc", e)
try:
    bytes("x", "bogus")
except LookupError as e:
    print("codec", e)
try:
    bytes("é", "ascii")
except UnicodeEncodeError as e:
    print("encode", e)
try:
    bytearray(b"a")[0] = 256
except ValueError as e:
    print("setrange", e)
try:
    bytearray(b"a").extend(5)
except TypeError as e:
    print("extend", e)
try:
    bytearray().pop()
except IndexError as e:
    print("popempty", e)
try:
    hash(bytearray(b"a"))
except TypeError as e:
    print("hash", e)
