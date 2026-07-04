# Augmented assignment mutates mutable builtins in place and calls the
# in-place dunder on user objects, so aliases observe the change.

# list += extends in place, alias-visible; accepts any iterable.
a = [1]
b = a
a += [2, 3]
print(a, b, a is b)
a += (4, 5)
print(a)
a += "xy"
print(a)

# list *= repeats in place.
c = [1, 2]
d = c
c *= 3
print(c, c is d)
c *= 0
print(c)

# a += a reads the original length, not the growing slice.
e = [1, 2]
e += e
print(e)

# set |= &= ^= -= update in place.
s = {1, 2}
t = s
s |= {3, 4}
print(sorted(s), s is t)
s &= {2, 3}
print(sorted(s), s is t)
s ^= {3, 9}
print(sorted(s), s is t)
s -= {2}
print(sorted(s), s is t)

# frozenset is immutable: |= rebinds to a fresh object.
f = frozenset({1, 2})
g = f
f |= {3}
print(sorted(f), f is g)

# Immutable numbers and strings rebind.
n = 1
m = n
n += 2
print(n, m, n is m)
p = "a"
q = p
p += "b"
print(p, q, p is q)
r = 2
r **= 10
r //= 3
print(r)

# Subscript and attribute targets share the receiver and mutate in place.
grid = [[1], [2]]
row = grid[0]
grid[0] += [8, 9]
print(grid, row is grid[0])


class Box:
    def __init__(self, items):
        self.items = items


box = Box([1])
kept = box.items
box.items += [7]
print(box.items, kept is box.items)


# A user in-place dunder returning self keeps aliases together.
class Acc:
    def __init__(self, total):
        self.total = total

    def __iadd__(self, other):
        self.total += other
        return self

    def __repr__(self):
        return f"Acc({self.total})"


x = Acc(10)
y = x
x += 5
print(x, y, x is y)


# __iadd__ returning NotImplemented falls back to __add__, which builds a new
# object, so the alias diverges.
class Money:
    def __init__(self, cents):
        self.cents = cents

    def __iadd__(self, other):
        return NotImplemented

    def __add__(self, other):
        return Money(self.cents + other)

    def __repr__(self):
        return f"Money({self.cents})"


u = Money(1)
v = u
u += 4
print(u, v, u is v)


# A class with only __add__ still works, rebinding to the new value.
class Tag:
    def __init__(self, name):
        self.name = name

    def __add__(self, other):
        return Tag(self.name + other)

    def __repr__(self):
        return f"Tag({self.name!r})"


w = Tag("a")
w += "b"
print(w)


# __imatmul__ drives @= on a user object.
class Vec:
    def __init__(self, v):
        self.v = v

    def __imatmul__(self, other):
        self.v = self.v + other.v
        return self

    def __repr__(self):
        return f"Vec({self.v})"


vec = Vec(3)
vec @= Vec(4)
print(vec)


# Error paths: the fallback names the augmented symbol; builtin messages stay.
class Bare:
    pass


try:
    bad = Bare()
    bad += 1
except TypeError as err:
    print("bare:", err)

try:
    lst = [1]
    lst += 5
except TypeError as err:
    print("list+int:", err)

try:
    num = 1
    num += "z"
except TypeError as err:
    print("int+str:", err)

try:
    text = "a"
    text += 1
except TypeError as err:
    print("str+int:", err)

try:
    seq = [1]
    seq *= "z"
except TypeError as err:
    print("list*str:", err)

# The tuple quirk: t[0] += [x] mutates the stored list in place and then the
# store-back to the immutable tuple raises, so the list still grows.
tup = ([1],)
try:
    tup[0] += [2]
except TypeError as err:
    print("tuple-quirk:", err)
print(tup)
