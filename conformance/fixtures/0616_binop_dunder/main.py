class Vec:
    def __init__(self, x, y):
        self.x = x
        self.y = y
    def __matmul__(self, o):
        return self.x * o.x + self.y * o.y
    def __add__(self, o):
        return Vec(self.x + o.x, self.y + o.y)
    def __mul__(self, k):
        return Vec(self.x * k, self.y * k)
    def __rmul__(self, k):
        return Vec(self.x * k, self.y * k)

a = Vec(1, 2)
b = Vec(3, 4)
print(a @ b)
c = a + b
print(c.x, c.y)
d = a * 3
print(d.x, d.y)
e = 5 * a
print(e.x, e.y)

# @= with no __imatmul__ falls back to __matmul__ like CPython
class Cat:
    def __init__(self, s):
        self.s = s
    def __matmul__(self, o):
        return Cat(self.s + o.s)
g = Cat("ab")
g @= Cat("cd")
print(g.s)

# a dunder returning NotImplemented on both sides raises unsupported-operand
class Decline:
    def __add__(self, o):
        return NotImplemented
    def __radd__(self, o):
        return NotImplemented
p = Decline()
q = Decline()
try:
    p + q
except TypeError as err:
    print("add:", err)
try:
    1 @ 2
except TypeError as err:
    print("matmul:", err)

# reflected on a plain-plus-instance pair, and the subclass-reflected-first rule
class Base:
    def __sub__(self, o):
        return "base.sub"
    def __rsub__(self, o):
        return "base.rsub"
class Sub(Base):
    def __rsub__(self, o):
        return "sub.rsub"
print(Base() - Sub())

class P:
    def __rpow__(self, o):
        return o + 100
print(2 ** P())
