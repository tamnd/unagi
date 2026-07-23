# id() returns an object's identity as an int, stable for the life of the
# object and equal exactly when two names refer to the same object. reprlib and
# other stdlib modules read id at import to key recursion guards, so the builtin
# has to resolve as both a value passed to map and a direct call.

a = [1, 2, 3]
b = a
print(type(id(a)).__name__)
print(id(a) >= 0)
print(id(a) == id(b))
print(a is b, id(a) == id(b))
print(id(a) == id([1, 2, 3]))
print(id(None) == id(None))
print(id(True) == id(True))
print(id(1) == id(1))
c = object()
print(id(c) == id(c))
ids = list(map(id, [a, b, c]))
print(ids[0] == ids[1], ids[0] != ids[2])
try:
    id()
except TypeError as e:
    print("TE:", e)
try:
    id(1, 2)
except TypeError as e:
    print("TE:", e)
