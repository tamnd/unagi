import fractions
import decimal

F = fractions.Fraction
D = decimal.Decimal


def show(label, fn):
    try:
        v = fn()
        print(label, "=>", type(v).__name__, repr(v))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# float.from_number and complex.from_number are the Python 3.14 classmethods that
# build the number from another number. Unlike the float() and complex()
# constructors they refuse a string, so a str, bytes or non-number raises
# "must be real number, not X" rather than being parsed. int gained no such
# classmethod, so int.from_number stays absent.
print("== float.from_number takes a real number ==")
show("from int", lambda: float.from_number(5))
show("from float", lambda: float.from_number(2.5))
show("from bool", lambda: float.from_number(True))
show("from Fraction", lambda: float.from_number(F(1, 2)))
show("from Decimal", lambda: float.from_number(D("1.5")))
show("from __index__", lambda: float.from_number(type("Y", (), {"__index__": lambda s: 9})()))
show("from __float__", lambda: float.from_number(type("Z", (), {"__float__": lambda s: 1.5})()))

print("== float.from_number rejects a non-real ==")
show("from str", lambda: float.from_number("2.5"))
show("from bytes", lambda: float.from_number(b"1"))
show("from bytearray", lambda: float.from_number(bytearray(b"1")))
show("from list", lambda: float.from_number([]))
show("from None", lambda: float.from_number(None))
show("from complex", lambda: float.from_number(1j))
show("bad __float__ return", lambda: float.from_number(type("Q", (), {"__float__": lambda s: "x"})()))

print("== complex.from_number takes any number ==")
show("from int", lambda: complex.from_number(3))
show("from float", lambda: complex.from_number(2.5))
show("from complex", lambda: complex.from_number(1 + 2j))
show("from bool", lambda: complex.from_number(False))
show("from Fraction", lambda: complex.from_number(F(1, 2)))
show("from __complex__", lambda: complex.from_number(type("C", (), {"__complex__": lambda s: 1 + 1j})()))
show("from __float__", lambda: complex.from_number(type("Cf", (), {"__float__": lambda s: 2.0})()))
show("from __index__", lambda: complex.from_number(type("Ci", (), {"__index__": lambda s: 5})()))

print("== complex.from_number rejects a non-number ==")
show("from str", lambda: complex.from_number("1"))
show("from bytes", lambda: complex.from_number(b"1"))
show("from None", lambda: complex.from_number(None))

print("== the arity and keyword errors ==")
show("float no arg", lambda: float.from_number())
show("float two args", lambda: float.from_number(1, 2))
show("float keyword", lambda: float.from_number(number=1))
show("complex no arg", lambda: complex.from_number())
show("complex two args", lambda: complex.from_number(1, 2))
show("complex keyword", lambda: complex.from_number(number=1))

print("== a subclass rebuilds itself ==")


class MyFloat(float):
    pass


class MyComplex(complex):
    pass


show("MyFloat.from_number", lambda: MyFloat.from_number(3))
show("MyComplex.from_number", lambda: MyComplex.from_number(3))

print("== int gained no from_number, the callables introspect as builtins ==")
print("hasattr int", hasattr(int, "from_number"))
print("hasattr float", hasattr(float, "from_number"))
print("hasattr complex", hasattr(complex, "from_number"))
print("float method name", float.from_number.__name__)
print("float method type", type(float.from_number).__name__)
print("complex method name", complex.from_number.__name__)
