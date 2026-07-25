# list.sort() routes through the full rich-comparison protocol, matching
# sorted(): an element type that defines only __lt__/__eq__ sorts in place, a
# functools.cmp_to_key wrapper key works, and reverse keeps stable order.
import functools


@functools.total_ordering
class N:
    def __init__(self, v):
        self.v = v

    def __eq__(self, other):
        return self.v == other.v

    def __lt__(self, other):
        return self.v < other.v

    def __repr__(self):
        return f"N({self.v})"


xs = [N(3), N(1), N(2)]
xs.sort()
print(xs)

xs.sort(reverse=True)
print(xs)


def cmp(a, b):
    return (a > b) - (a < b)


ys = [3, 1, 2, 5, 4]
ys.sort(key=functools.cmp_to_key(cmp))
print(ys)

ys.sort(key=functools.cmp_to_key(cmp), reverse=True)
print(ys)

zs = ["bb", "a", "ccc"]
zs.sort(key=len)
print(zs)

# Stability: equal keys keep input order.
pairs = [(1, "a"), (0, "b"), (1, "c"), (0, "d")]
pairs.sort(key=lambda p: p[0])
print(pairs)
