# float, complex, bytes, bytearray and str expose their operator, unary,
# conversion and pickle-support dunders off the type as unbound descriptors, so
# float.__add__(1.0, 2.0) and str.__mul__("a", 3) dispatch the way the bound
# (1.0).__add__(2.0) and "a".__mul__(3) do. This is the type-object side of the
# instance-dunder surface those types already carry, matching the way int and bool
# already read their operator slots off the type. The first argument is the
# instance, so a missing or wrong-typed one raises the descriptor's own TypeErrors.


def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as e:
        print(label, "ERR", type(e).__name__, str(e))


# hasattr answers True across the operator surface each type carries.
print("float has:", [n for n in ("__add__", "__mul__", "__abs__", "__neg__", "__getnewargs__") if hasattr(float, n)])
print("str has:", [n for n in ("__add__", "__mul__", "__rmul__", "__mod__", "__getnewargs__") if hasattr(str, n)])
print("bytes has:", [n for n in ("__add__", "__mul__", "__mod__", "__bytes__", "__getnewargs__") if hasattr(bytes, n)])

# The operator dunders read off the type and dispatch through the argument's slot.
show("float.__add__(1.,2.)", lambda: float.__add__(1.0, 2.0))
show("float.__mul__(2.,3.)", lambda: float.__mul__(2.0, 3.0))
show("float.__truediv__(1.,2.)", lambda: float.__truediv__(1.0, 2.0))
show("float.__neg__(2.5)", lambda: float.__neg__(2.5))
show("float.__abs__(-2.5)", lambda: float.__abs__(-2.5))
show("float.__getnewargs__(1.5)", lambda: float.__getnewargs__(1.5))
show("complex.__add__(1j,2j)", lambda: complex.__add__(1j, 2j))
show("complex.__mul__(2j,3j)", lambda: complex.__mul__(2j, 3j))
show("complex.__abs__(3+4j)", lambda: complex.__abs__(3 + 4j))
show("str.__add__('a','b')", lambda: str.__add__("a", "b"))
show("str.__mul__('ab',3)", lambda: str.__mul__("ab", 3))
show("str.__mod__('%d',5)", lambda: str.__mod__("%d", 5))
show("str.__getnewargs__('x')", lambda: str.__getnewargs__("x"))
show("bytes.__add__(b'a',b'b')", lambda: bytes.__add__(b"a", b"b"))
show("bytes.__mul__(b'a',3)", lambda: bytes.__mul__(b"a", 3))
show("bytes.__bytes__(b'x')", lambda: bytes.__bytes__(b"x"))
show("bytearray.__add__(bytearray(b'a'),b'b')", lambda: bytearray.__add__(bytearray(b"a"), b"b"))
show("bytearray.__iadd__(bytearray(b'a'),b'b')", lambda: bytearray.__iadd__(bytearray(b"a"), b"b"))

# The descriptor rejects a missing or wrong-typed first argument, and the bound
# slot owns the remaining-argument arity error.
show("float.__add__()", lambda: float.__add__())
show("float.__add__(1.)", lambda: float.__add__(1.0))
show("float.__add__('x',2)", lambda: float.__add__("x", 2))
show("str.__add__(3,'b')", lambda: str.__add__(3, "b"))
show("bytes.__mul__('x',3)", lambda: bytes.__mul__("x", 3))

# A type only exposes the slots its own instances carry: bytes has no true divide.
print("bytes has __truediv__:", hasattr(bytes, "__truediv__"))
print("float has __getitem__:", hasattr(float, "__getitem__"))

# The type-level descriptor uses the base type's slot even on a subclass instance.


class MyStr(str):
    pass


show("str.__add__(MyStr('a'),'b')", lambda: str.__add__(MyStr("a"), "b"))

# bool passes as an int, so int's type-level slots stay unchanged and reachable.
show("int.__add__(1,2)", lambda: int.__add__(1, 2))
