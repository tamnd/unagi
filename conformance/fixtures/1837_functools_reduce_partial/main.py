# functools.reduce and functools.partial, the two C-backed callables the runtime
# owns behind the functools import. reduce folds a binary function over an
# iterable, with and without an initializer, and raises on an empty iterable with
# no seed. partial freezes leading positionals and keywords of a callable,
# exposes func/args/keywords, flattens a partial over a partial, and lets a later
# call override a frozen keyword. Reprs use builtins so the text is stable.
import functools
from functools import reduce, partial

print(reduce(lambda a, b: a + b, [1, 2, 3, 4]))
print(reduce(lambda a, b: a + b, [1, 2, 3, 4], 100))
print(reduce(lambda a, b: a * b, range(1, 5)))
print(reduce(lambda a, b: a, [], 42))


def f(a, b, c):
    return (a, b, c)


p = partial(f, 1, c=3)
print(p(2))
print(p.func is f, p.args, p.keywords)
print(p(2, c=9))

q = partial(f, 1)
print(q(2, 3))

r = partial(partial(f, 1), 2)
print(r(3))
print(r.func is f, r.args)


def g(a=0, b=0):
    return (a, b)


print(partial(g, a=1, b=2)(b=9))

print(repr(partial(len)))
print(repr(partial(pow, 2)))
print(partial(pow, 2)(3))


def show(fn):
    try:
        return fn()
    except Exception as e:
        return type(e).__name__ + " " + str(e)


print(show(lambda: reduce(lambda a, b: a, [])))
print(show(lambda: reduce(lambda a, b: a)))
print(show(lambda: reduce(lambda a, b: a, [1], 2, 3)))
print(show(lambda: partial(1)))
print(show(lambda: partial()))
print(show(lambda: partial(f, 1).missing))
