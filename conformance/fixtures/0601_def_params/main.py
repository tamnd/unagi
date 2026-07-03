# Full def parameter surface: defaults, keyword calls, positional-only,
# keyword-only, *args and **kwargs, all bound at compile time.

def greet(name, greeting="hello", punct="!"):
    return greeting + ", " + name + punct

print(greet("ana"))
print(greet("bo", "hi"))
print(greet("cy", punct="?"))
print(greet(punct=".", name="di", greeting="yo"))

def posonly(a, b, /, c):
    return a * 100 + b * 10 + c

print(posonly(1, 2, 3))
print(posonly(4, 5, c=6))

def kwonly(a, *, k, m=9):
    return a * 100 + k * 10 + m

print(kwonly(1, k=2))
print(kwonly(1, k=2, m=3))
print(kwonly(m=4, a=5, k=6))

def variadic(a, *rest, **extra):
    return (a, rest, extra)

print(variadic(1))
print(variadic(1, 2, 3))
print(variadic(1, 2, x=3, y=4))

def mixed(a, b=2, *args, k, m=5, **kw):
    return (a, b, args, k, m, kw)

print(mixed(1, k=3))
print(mixed(1, 9, 8, 7, k=3, m=4, z=6))

# Defaults evaluate once, when the def statement runs.
base = 10

def snap(v=base):
    return v

base = 20
print(snap())
print(snap(99))

# A mutable default is shared across calls.
def accum(x, box=[]):
    box.append(x)
    return box

print(accum(1))
print(accum(2))
print(accum(3, []))
print(accum(4))

# Default expressions can call earlier defs.
def double(n):
    return n * 2

def uses(v=double(21)):
    return v

print(uses())
