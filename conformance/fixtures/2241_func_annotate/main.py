# PEP 649: a function carries a lazy __annotate__ mirroring the class-side one.
import annotationlib
import functools

VALUE = annotationlib.Format.VALUE


def f(x: int, y: str) -> bool: ...
def plain(a, b): ...


# __annotate__ is a callable exactly when the def declared annotations, else None.
print("annotated has annotate", f.__annotate__ is not None)
print("plain annotate", plain.__annotate__)

# The callable returns the {name: type} mapping under the VALUE format.
print("value", f.__annotate__(VALUE))

# Only VALUE is supported; the compiler-emitted annotate raises for the other
# formats and annotationlib synthesizes them via its fake-globals fallback.
for fmt in (annotationlib.Format.FORWARDREF, annotationlib.Format.STRING):
    try:
        f.__annotate__(fmt)
        print("no raise", int(fmt))
    except NotImplementedError:
        print("nie", int(fmt))

# Reading __annotations__ first (which realizes the deferred thunks) leaves a
# later __annotate__ read intact.
def g(n: int) -> str: ...
_ = g.__annotations__
print("post-realize", g.__annotate__(VALUE))

# Each call hands back a fresh dict, so a mutation never leaks into
# __annotations__.
d = f.__annotate__(VALUE)
d["injected"] = 1
print("fresh dict", "injected" not in f.__annotations__)

# A method and a return-only annotation resolve the same way.
class C:
    def m(self, n: int) -> None: ...
    def p(self): ...


print("method", C.m.__annotate__(VALUE))
print("plain method", C.p.__annotate__)


def ret() -> int: ...
print("return only", ret.__annotate__(VALUE))

# annotationlib reads the annotate off a function to recover its annotations.
print("get_annotations", annotationlib.get_annotations(f, format=VALUE))

# The motivating case: functools.singledispatch dispatches on the bare
# @register of an annotated def, which needs __annotate__ to recover the type.
@functools.singledispatch
def kind(arg):
    return "default"


@kind.register
def _(arg: int):
    return "int"


@kind.register
def _(arg: str):
    return "str"


print("dispatch", kind(1), kind("x"), kind(1.0))
