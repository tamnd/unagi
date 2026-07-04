# Call-site * and ** unpacking: merge shapes, evaluation order, and the
# routes through user functions, builtins, methods, and exception
# constructors. Probed on 3.14.

def f(a, b, c=10, *rest, k=1, **extra):
    print(a, b, c, rest, k, extra)

t = [1, 2]
d = {"k": 5, "z": 9}
f(*t)
f(*t, 3, 4, **d)
f(0, *t, k=7)
f(*[], 1, 2)
f(**{"a": 1, "b": 2})
f(*t, **{})

# A star can follow a keyword; the positional group still evaluates first.
def mk(tag, v):
    print("eval", tag)
    return v

def spy(*args, **kw):
    print("call", args, kw)

spy(mk("a", 1), *mk("s", [2, 3]), mk("b", 4), x=mk("x", 5), **mk("d", {"y": 6}))
spy(x=mk("kw", 1), *mk("star", [7]))
spy(1, *[2], x=3, *[4])

# Any iterable unpacks: str, tuple, range, dict (its keys), set of one.
spy(*"ab", *(3,), *range(2), *{"p": 1}, *{9})

# Unpacking through a function value and a lambda.
g = f
g(*t, 3)
add = lambda a, b: a + b
print(add(*t))
print(add(*[8], **{"b": 4}))

# Builtins through the runtime table.
print(len(*[[10, 20, 30]]))
print(max(*t, 99))
print(min(*[5, 3]))
print(sorted(*[[3, 1, 2]]))
print(list(*[range(3)]))
print(dict(*[[("a", 1)]]))
print(sum(*[[1, 2, 3]], 100))
print(divmod(*t))
print(*t, *[3])

# Methods take a lone star.
lst = [3, 1]
lst.extend(*[[2, 0]])
print(lst)
print("-".join(*[["a", "b"]]))

# Exception constructors unpack too.
e = ValueError(*t)
print(e)
try:
    raise IndexError(*["boom"])
except IndexError as exc:
    print(exc)
