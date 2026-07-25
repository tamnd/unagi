# inspect.signature over a class reads the constructor. It binds __init__ or
# __new__ off the type through the function descriptor protocol, strips the
# leading self/cls, and reports the remaining parameters. A subclass with no
# constructor of its own inherits the one it found up the MRO.
import inspect


class C:
    def __init__(self, a, b=1, *c, d, **e):
        pass


class E:
    def __new__(cls, x, y=5):
        return object.__new__(cls)


class Sub(C):
    pass


print(inspect.signature(C))
print(inspect.signature(E))
print(inspect.signature(Sub))

# The Signature object exposes its parameters, so a caller can read a default
# and a kind without going through the repr.
sig = inspect.signature(C)
print(list(sig.parameters))
print(sig.parameters["b"].default)
print(sig.parameters["c"].kind == inspect.Parameter.VAR_POSITIONAL)
print(sig.parameters["d"].kind == inspect.Parameter.KEYWORD_ONLY)
