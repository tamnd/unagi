print(hash(0), hash(1), hash(-1), hash(2**200), hash(-(2**61 + 5)))
print(hash(1.5), hash(0.5), hash(float("inf")), hash(-0.0), hash(2.0**61))
print(hash(None), hash(True), hash(False))
print(hash(""), hash("a"), hash("abc"), hash("héllo"), hash("日本"), hash("😀"))
print(hash(()), hash((1, 2)), hash(("a", 1.5, None)), hash(((1, 2), (3,))))
print(hash(frozenset()), hash(frozenset([1, 2, 3])), hash(frozenset(["a", 2.5])))
print(hash(range(5)), hash(range(0)), hash(range(1, 10, 2)))
print(hash(1) == hash(1.0), hash(-2) == hash(-2.0), hash(2**61) == hash(2.0**61))
try:
    hash([1])
except TypeError as e:
    print("uh:", e)
try:
    hash({1})
except TypeError as e:
    print("uh2:", e)
try:
    hash({1: 2})
except TypeError as e:
    print("uh3:", e)
try:
    hash((1, [2]))
except TypeError as e:
    print("uh4:", e)
