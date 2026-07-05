# PEP 479: a StopIteration that escapes a generator frame must not leak out as
# ordinary exhaustion. It becomes RuntimeError("generator raised StopIteration")
# carrying the original StopIteration as both __cause__ and __context__, with
# the context suppressed. A normal return still exhausts cleanly.


def raises_stop():
    raise StopIteration("escaped")
    yield 1


try:
    list(raises_stop())
except RuntimeError as e:
    print("direct:", e)
    print("cause:", type(e.__cause__).__name__, e.__cause__)
    print("context is cause:", e.__context__ is e.__cause__)
    print("suppress:", e.__suppress_context__)


# A StopIteration from an exhausted inner next() inside a generator converts too.
def inner_next():
    it = iter([])
    next(it)
    yield 1


try:
    list(inner_next())
except RuntimeError as e:
    print("inner-next:", e)


# The conversion applies through a yield-from delegation as well.
def sub():
    raise StopIteration("inner")
    yield 1


def deleg():
    yield from sub()


try:
    list(deleg())
except RuntimeError as e:
    print("delegated:", e)


# A generator that returns normally exhausts cleanly, no conversion.
def clean():
    yield 1
    yield 2
    return


print("clean:", list(clean()))
