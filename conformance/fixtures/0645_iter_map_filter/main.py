# iter(o) hands back an iterator that next() and a for loop share.
it = iter([1, 2, 3])
print(next(it), next(it), next(it))
print(next(it, "done"))
for x in iter(("a", "b")):
    print("loop", x)

# iter(callable, sentinel) calls until the sentinel shows up.
data = iter([1, 2, 0, 3])
print(list(iter(lambda: next(data), 0)))

# map is lazy: the function only runs as the result is pulled.
def double(x):
    print("call", x)
    return x * 2

m = map(double, [1, 2, 3])
print("made map")
print("first", next(m))
print(list(m))

# Several iterables stop at the shortest one.
print(list(map(lambda a, b: a + b, [1, 2, 3], [10, 20])))

# filter is lazy too, and filter(None, ...) keeps the truthy elements.
def odd(x):
    print("pred", x)
    return x % 2

fl = filter(odd, [1, 2, 3, 4])
print("first", next(fl))
print(list(fl))
print(list(filter(None, [0, 1, "", 2, None, 3])))

# The catalog of catchable errors.
try:
    iter(5)
except TypeError as e:
    print("T", e)
try:
    iter(5, 0)
except TypeError as e:
    print("nc", e)
try:
    map(double)
except TypeError as e:
    print("m1", e)
try:
    filter(odd)
except TypeError as e:
    print("f1", e)
