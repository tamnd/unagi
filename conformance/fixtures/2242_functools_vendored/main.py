# The public functools now runs from the vendored pure-Python module rather
# than a Go shim: exercise the full surface it exposes.
import functools

# reduce, with and without an initializer.
print(functools.reduce(lambda a, b: a + b, [1, 2, 3, 4]))
print(functools.reduce(lambda a, b: a + b, [1, 2, 3], 100))

# partial freezes leading positionals and keywords and reprs as a class of
# module functools with the bare name "partial".
p = functools.partial(lambda a, b, c: (a, b, c), 1, c=3)
print(p(2), type(p).__name__, type(p).__module__)
print(p.func is not None, p.args, sorted(p.keywords))

# Placeholder leaves a positional hole a later call fills.
ph = functools.Placeholder
skip = functools.partial(lambda a, b: (a, b), ph, "y")
print(skip("x"))

# lru_cache memoizes and reports hits/misses through cache_info.
@functools.lru_cache(maxsize=2)
def sq(n):
    return n * n


print(sq(2), sq(2), sq(3), sq(4), sq(3))
info = sq.cache_info()
print(info.hits, info.misses, info.maxsize, info.currsize)
sq.cache_clear()
print(sq.cache_info().currsize)


# cache is the unbounded shorthand.
@functools.cache
def cube(n):
    return n ** 3


print(cube(3), cube(3))

# cmp_to_key adapts an old-style comparison for sorted.
print(sorted([3, 1, 2], key=functools.cmp_to_key(lambda a, b: a - b)))


# total_ordering fills in the derived comparisons.
@functools.total_ordering
class Num:
    def __init__(self, v):
        self.v = v

    def __eq__(self, o):
        return self.v == o.v

    def __lt__(self, o):
        return self.v < o.v


print(Num(1) < Num(2), Num(2) >= Num(2), Num(3) > Num(1))


# singledispatch dispatches on the first argument's type, including the bare
# @register on an annotated def (which reads the type off PEP 649 __annotate__).
@functools.singledispatch
def kind(x):
    return "object"


@kind.register
def _(x: int):
    return "int"


@kind.register(str)
def _(x):
    return "str"


print(kind(1), kind("a"), kind(1.0))


# singledispatchmethod dispatches a method on its first non-self argument.
class Handler:
    @functools.singledispatchmethod
    def go(self, arg):
        return "default"

    @go.register
    def _(self, arg: int):
        return "int"


h = Handler()
print(h.go(1), h.go("x"))


# wraps / update_wrapper copy identity onto a wrapper.
def deco(fn):
    @functools.wraps(fn)
    def w(*a, **k):
        return fn(*a, **k)

    return w


@deco
def orig(x):
    "the original docstring"
    return x


print(orig.__name__, orig.__doc__, orig.__wrapped__ is not None)


# cached_property computes once per instance and caches in the instance dict.
class Box:
    def __init__(self):
        self.hits = 0

    @functools.cached_property
    def value(self):
        self.hits += 1
        return 42


b = Box()
print(b.value, b.value, b.hits)


# partialmethod binds frozen arguments to a method.
class Adder:
    def _add(self, a, b):
        return a + b

    add10 = functools.partialmethod(_add, 10)


print(Adder().add10(5))

# The module exposes its wrapper-assignment constants.
print("__name__" in functools.WRAPPER_ASSIGNMENTS)
print(functools.reduce is not None)
