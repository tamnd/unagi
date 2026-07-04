# Bytes literals: the b prefix, repr, indexing, slicing, iteration and the
# core operators. Every value below prints identically under CPython 3.14.
b = b"hello\x00\ttab\n\\end"
print(b)
print(repr(b))
print(str(b))

# Prefix folding and raw mode keep backslashes literal.
print(B"upper", rb"C:\new", br"raw\t", Rb"a\z", bR"b\q")

# Indexing yields ints; slicing yields bytes.
print(b"abcde"[0], b"abcde"[-1])
print(b"abcde"[1:4], b"abcde"[::-1], b"abcde"[::2])
print([x for x in b"abc"])
print(len(b"hello"), len(b""))

# Concatenation, repetition and adjacency.
print(b"ab" + b"cd")
print(b"ab" * 3, 3 * b"xy", b"ab" * -1)
print(b"one" b"two")

# Membership: subsequence and member byte.
print(b"at" in b"cat", 97 in b"cat", 120 in b"cat")

# Equality never crosses into str; ordering is lexicographic.
print(b"abc" == b"abc", b"abc" == "abc", b"abc" == 97)
print(b"abc" < b"abd", b"b" > b"a", b"ab" < b"abc")

# Truthiness follows length.
print(bool(b""), bool(b"x"))

# Bytes are hashable, so they key sets and dicts.
seen = {b"a", b"b", b"a"}
print(len(seen))
table = {b"k": 1, b"v": 2}
print(table[b"k"], table[b"v"])

# Escape catalog: hex, octal, control bytes.
print(list(b"\x41\102\a\b\f\v\0"))
print(list(b"\376\77"))

# Catchable errors, one per labelled line.
try:
    b"a" + "b"
except TypeError as e:
    print("concat", e)
try:
    "a" + b"b"
except TypeError as e:
    print("rconcat", e)
try:
    b"a" < 5
except TypeError as e:
    print("order", e)
try:
    b"abc"[9]
except IndexError as e:
    print("index", e)
try:
    256 in b"a"
except ValueError as e:
    print("member", e)
try:
    "a" in b"abc"
except TypeError as e:
    print("wrongtype", e)
