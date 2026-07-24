# A Python function exposes its metadata as __code__, a code object. unagi keeps
# no bytecode, so co_code and the constant and name pools are out of scope and
# the source location (co_filename, co_firstlineno) is not carried, but the
# argument shape is faithful: the counts, co_varnames and co_flags come from the
# declared parameters. importlib/_bootstrap_external reads type(func.__code__)
# at import, so this is the last gap before `import importlib` completes.


def full(a, b, /, c, d=1, *args, e, f=2, **kw):
    pass


co = full.__code__
print(type(co).__name__)
print(co.co_name, co.co_qualname)
print(co.co_argcount, co.co_posonlyargcount, co.co_kwonlyargcount)
print(co.co_flags)
print(co.co_varnames)


def plain(x, y):
    pass


pco = plain.__code__
print(pco.co_argcount, pco.co_flags, pco.co_varnames)


# A method's qualname carries the class, and self counts as an argument.
class C:
    def method(self, x):
        pass


mco = C.method.__code__
print(mco.co_name, mco.co_qualname, mco.co_argcount)

# A lambda is a function too.
lam = lambda p, q=0: p
print(lam.__code__.co_name, lam.__code__.co_argcount, lam.__code__.co_varnames)

# co_flags separates *args (0x04) from **kwargs (0x08).
star = lambda *a: a
starstar = lambda **k: k
print(star.__code__.co_flags, starstar.__code__.co_flags)

# The payoff: the whole importlib bootstrap now imports.
import importlib
import importlib.machinery

print(importlib.__name__)
print(hasattr(importlib.machinery, "ModuleSpec"))
