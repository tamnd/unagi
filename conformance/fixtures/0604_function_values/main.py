# Functions as first-class values: assignment, passing, returning, calling
# through variables, identity semantics. Wordings probed on 3.14.

try:
    print(f)
except NameError as e:
    print(e)

def f(x, y=10):
    return x + y

print(f(1))
g = f
print(g(2), g(2, 3), g(2, y=4))
print(repr(g))
print(g is f, g == f, bool(g))

def apply(fn, v):
    return fn(v)

print(apply(f, 5))
print(apply(g, 6))

def base(x):
    return x * 100

def pick(flag):
    return base

h = pick(True)
print(h(7))

d = {f: "one"}
d[g] = "two"
print(d[f], len(d))

f = 3
try:
    f(1)
except TypeError as e:
    print(e)
print(g(1), f + 1)
f = g
print(f(1, y=0))
