# Closure reads: late binding over module and function scope, unbound free
# variables, and the traceback a lambda frame prints. Probed on 3.14.

x = 1
get = lambda: x
x = 2
print(get())
del x
try:
    get()
except NameError as e:
    print(e)
x = 3
print(get())

def factory():
    c = 10
    r = lambda: c
    c = 11
    return r

print(factory()())

def unbound():
    r = lambda: z
    v = r()
    z = 1
    return v

try:
    unbound()
except NameError as e:
    print(e)

later = lambda: q
try:
    later()
except NameError as e:
    print(e)
q = 40
print(later())

def chain(a):
    return lambda b: lambda c: a + b + c

print(chain(1)(2)(3))

boom = lambda d: 1 // d
print(boom(1))
boom(0)
