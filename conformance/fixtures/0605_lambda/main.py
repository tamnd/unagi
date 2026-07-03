# Lambda expressions: parameter grammar, defaults, qualnames, walrus
# scoping. Wordings probed on 3.14.

f = lambda: 1
print(f(), repr(f))
double = lambda x: x * 2
print(double(21))

full = lambda a, b=2, /, c=3, *rest, k, kk=5, **kw: (a, b, c, rest, k, kk, kw)
print(full(1, k=4))
print(full(1, 2, 3, 9, 8, k=4, kk=6, z=7))

n = 5
scale = lambda v: v * n
n = 7
print(scale(2))

base = 10
snap = lambda v=base: v
base = 20
print(snap(), snap(99))

def make(tag):
    return lambda x: (tag, x)

a = make("a")
b = make("b")
print(a(1), b(2))
print(repr(a))

print((lambda x: lambda y: x + y)(30)(12))
print((lambda *xs: sum(xs))(1, 2, 3))

w = (lambda v: (u := v + 1) and u * 2)(5)
print(w)
pickd = lambda z=(m := 100): (z, m)
print(pickd(), m)

t = lambda: 1, 2
print(t[0](), t[1])
cond = lambda x: "yes" if x else "no"
print(cond(1), cond(0))
