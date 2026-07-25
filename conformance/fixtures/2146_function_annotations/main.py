# Function parameter and return annotations populate __annotations__, evaluated
# lazily on the first read the way PEP 649 defers them, so a forward reference or
# a name bound only later costs nothing at definition time. functools's
# singledispatch reads them to pick a registration type.

# A simple signature: the values evaluate to the annotated types, "return" last.
def basic(x: int, y: "str", z=3) -> bool:
    return True


print(basic.__annotations__)


# A forward reference to a name defined later evaluates lazily, only on read.
def uses_later(x: "Later") -> "Later":
    return x


class Later:
    pass


print(uses_later.__annotations__)


# *args, **kwargs and keyword-only annotations, in declaration order.
def spread(a: int, b: str = "x", *args: float, c: bool = True, **kw: bytes):
    return None


print(spread.__annotations__)


# A method carries its own annotations.
class C:
    def m(self, n: int) -> str:
        return str(n)


print(C.m.__annotations__)


# A nested def too.
def outer():
    def inner(z: float) -> int:
        return int(z)

    return inner


print(outer().__annotations__)


# An unannotated def has an empty, mutable annotations dict.
def plain(x, y):
    return x


print(plain.__annotations__)
plain.__annotations__["x"] = int
print(plain.__annotations__)


# Reading twice hands back the same dict; a mutation sticks.
d1 = uses_later.__annotations__
d1["extra"] = 5
print(uses_later.__annotations__["extra"])
print(uses_later.__annotations__ is d1)


# Assigning __annotations__ replaces it.
def g(p: int):
    return p


g.__annotations__ = {"p": str}
print(g.__annotations__)


# An unresolved annotation raises only on read, not at definition.
def bad(x: "NeverDefined"):
    return x


try:
    bad.__annotations__
except NameError:
    print("NameError")


# functools.singledispatch reads a registration's annotation to pick the type.
import functools


@functools.singledispatch
def area(shape):
    return "unknown"


@area.register
def _(shape: int):
    return "int"


@area.register
def _(shape: str):
    return "str"


print(area(1))
print(area("a"))
print(area(1.0))
