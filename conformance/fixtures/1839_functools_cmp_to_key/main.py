# functools.cmp_to_key, the bridge from an old-style comparison function to a
# key function. cmp_to_key(cmp) returns a wrapper; sorted calls it on each
# element and orders the resulting wrappers through their rich comparisons, which
# call cmp and test the sign against zero. The wrapper compares to another
# wrapper only, exposes obj, and takes exactly one argument when bound.
import functools
from functools import cmp_to_key


def cmp(a, b):
    return (a > b) - (a < b)


data = [3, 1, 4, 1, 5, 9, 2, 6]
print(sorted(data, key=cmp_to_key(cmp)))
print(sorted(data, key=cmp_to_key(cmp), reverse=True))


def rcmp(a, b):
    return (b > a) - (b < a)


print(sorted(data, key=cmp_to_key(rcmp)))

words = ["ccc", "a", "bb", "dddd"]
print(sorted(words, key=cmp_to_key(lambda x, y: len(x) - len(y))))

# The wrapper's own rich comparisons, driven by cmp against zero.
K = cmp_to_key(cmp)
a = K(1)
b = K(2)
c = K(1)
print(a < b, a > b, a == b, a <= b, a >= b, a != b)
print(b < a, b > a)
print(a == c, a < c, a <= c)
print(a.obj, b.obj)

# A stable sort keeps equal-key elements in their original order.
pairs = [("a", 3), ("b", 1), ("c", 3), ("d", 1)]
print(sorted(pairs, key=cmp_to_key(lambda x, y: x[1] - y[1])))


def show(fn):
    try:
        return fn()
    except Exception as e:
        return type(e).__name__ + " " + str(e)


print(show(lambda: K(1) < 5))
print(show(lambda: K(1, 2)))
print(show(lambda: K()))
