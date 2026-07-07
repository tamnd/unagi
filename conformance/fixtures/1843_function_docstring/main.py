# A function's docstring, the leading bare string literal in its body, becomes
# the initial __doc__. A function with no such literal has __doc__ None. The
# docstring is an ordinary slot value, so a later assignment overrides it and a
# del reverts it to None rather than back to the docstring. Methods, nested
# functions, and generators carry their docstrings the same way.


def documented(x):
    "double the argument"
    return x * 2


def bare(x):
    return x * 2


print(documented.__doc__)
print(bare.__doc__)


class C:
    def method(self):
        "a method summary"
        return 1

    def plain(self):
        return 2


print(C.method.__doc__)
print(C.plain.__doc__)


def outer():
    def inner():
        "the inner one"
        return 0

    return inner


print(outer().__doc__)


def gen():
    "a generator function"
    yield 1


print(gen.__doc__)

# The docstring is the initial value of an ordinary slot.
documented.__doc__ = "changed"
print(documented.__doc__)
del documented.__doc__
print(documented.__doc__)

# A multi-line docstring keeps its exact text.
def multi():
    """first line
    second line"""
    return None


print(repr(multi.__doc__))
