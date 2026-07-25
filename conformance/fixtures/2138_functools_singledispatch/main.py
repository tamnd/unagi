from functools import singledispatch


@singledispatch
def fun(arg, verbose=False):
    if verbose:
        print("default:", end=" ")
    print(arg)


@fun.register(int)
def _(arg, verbose=False):
    if verbose:
        print("int:", end=" ")
    print(arg * 2)


@fun.register(list)
def _(arg, verbose=False):
    if verbose:
        print("list:", end=" ")
    print(len(arg))


@fun.register(float)
@fun.register(complex)
def _(arg, verbose=False):
    print("number", arg)


fun("hello")
fun(7)
fun(7, verbose=True)
fun([1, 2, 3])
fun([1, 2, 3], verbose=True)
fun(1.5)
fun(3.0)

# Dispatch reads: a registered type resolves to its own impl, an unregistered
# one falls back to the default.
print(fun.dispatch(int) is not fun.dispatch(str))
print(fun.dispatch(str) is fun.dispatch(object))
print(list(fun.registry.keys()) == [object, int, list, complex, float])

# A subclass without its own registration is served by a registered base.
class Base:
    pass


class Derived(Base):
    pass


@fun.register(Base)
def _(arg, verbose=False):
    print("base impl")


fun(Derived())

# The two-argument register(cls, func) form binds without decoration.
def plain(arg, verbose=False):
    print("plain", arg)


fun.register(str, plain)
fun("x")

# A bool is an int subclass, so it dispatches to the int impl.
fun(True)

# No positional argument has nothing to dispatch on.
try:
    fun()
except TypeError as e:
    print("TypeError:", e)

print(fun.__name__)
