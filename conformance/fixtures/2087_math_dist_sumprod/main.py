# math gains dist, the Euclidean distance between two points, and sumprod, the
# dot product of two iterables. Both are correctly rounded in CPython through a
# scaled or compensated algorithm, so on a general float the last bit can differ
# from a straightforward computation. The fixture asserts only the facts the two
# agree on exactly: Pythagorean-exact distances, exact integer dot products, and
# the zero, infinity, nan and error cases.

import math

# dist on points whose distance is a whole number, the classic Pythagorean
# triples, is exact.
print(math.dist([0, 0], [3, 4]))
print(math.dist([0, 0, 0], [2, 3, 6]))
print(math.dist([5.0], [2.0]))
print(math.dist([1, 1], [4, 5]))

# a point at zero distance from itself, and the empty point.
print(math.dist([1, 2, 3], [1, 2, 3]))
print(math.dist([], []))

# tuples work as well as lists.
print(math.dist((0, 0), (3, 4)))

# an infinite coordinate difference is an infinite distance, a nan is nan.
print(math.dist([math.inf, 0], [0, 0]))
print(math.isnan(math.dist([math.nan], [0.0])))

# sumprod is exact on integers, including arbitrarily large ones.
print(math.sumprod([1, 2, 3], [4, 5, 6]))
print(math.sumprod([10**20, 2], [3, 10**20]))
print(math.sumprod([], []))
print(math.sumprod([True, False], [10, 20]))

# a mixed product that lands on an exact float.
print(math.sumprod([1, 2], [0.5, 0.5]))

# the infinity and nan cases, including zero times infinity.
print(math.sumprod([math.inf], [2.0]))
print(math.isnan(math.sumprod([math.nan], [1.0])))
print(math.sumprod([0.0], [math.inf]))


def show(label, fn):
    try:
        print(label, '->', fn())
    except (TypeError, ValueError, OverflowError) as ex:
        print(label, '->', type(ex).__name__ + ':', ex)


# dist rejects points of different lengths, a missing argument and a non-point.
show('dist mismatch', lambda: math.dist([1, 2], [1, 2, 3]))
show('dist 1 arg', lambda: math.dist([1, 2]))
show('dist scalar', lambda: math.dist(1, 2))

# sumprod rejects unequal lengths, a missing argument and a non-iterable.
show('sumprod mismatch', lambda: math.sumprod([1, 2], [1, 2, 3]))
show('sumprod 1 arg', lambda: math.sumprod([1, 2]))
show('sumprod scalar', lambda: math.sumprod(1, 2))
