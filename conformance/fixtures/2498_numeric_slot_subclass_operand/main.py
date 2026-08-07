# A numeric base arithmetic slot accepts a numeric-subclass operand and reads it
# as its stored scalar, the way CPython's C slots pull the double or the int out
# of a subclass argument and return a plain result. An operand outside the domain
# still declines with NotImplemented so its own reflected method runs, and a
# subclass that overrides the slot keeps its own dispatch.


def show(label, fn):
    try:
        v = fn()
        print(label, "=>", type(v).__name__, repr(v))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


class MyInt(int):
    pass


class MyFloat(float):
    pass


class MyComplex(complex):
    pass


# float slots take an int, bool or float subclass operand as its scalar.
show("float.__add__(6.0, MyFloat(2.0))", lambda: float.__add__(6.0, MyFloat(2.0)))
show("float.__sub__(6.0, MyInt(2))", lambda: float.__sub__(6.0, MyInt(2)))
show("float.__mul__(1.5, MyInt(4))", lambda: float.__mul__(1.5, MyInt(4)))
show("(6.0).__radd__(MyFloat(2.0))", lambda: (6.0).__radd__(MyFloat(2.0)))
show("float.__floordiv__(7.0, MyInt(2))", lambda: float.__floordiv__(7.0, MyInt(2)))
show("float.__mod__(7.5, MyFloat(2.0))", lambda: float.__mod__(7.5, MyFloat(2.0)))
show("float.__divmod__(7.0, MyInt(2))", lambda: float.__divmod__(7.0, MyInt(2)))
show("float.__pow__(2.0, MyInt(3))", lambda: float.__pow__(2.0, MyInt(3)))

# int slots take an int or bool subclass operand as its int.
show("int.__add__(6, MyInt(2))", lambda: int.__add__(6, MyInt(2)))
show("int.__mul__(6, MyInt(2))", lambda: int.__mul__(6, MyInt(2)))
show("int.__and__(6, MyInt(3))", lambda: int.__and__(6, MyInt(3)))
show("int.__lshift__(1, MyInt(4))", lambda: int.__lshift__(1, MyInt(4)))
show("(6).__rsub__(MyInt(2))", lambda: (6).__rsub__(MyInt(2)))
show("int.__pow__(2, MyInt(5))", lambda: int.__pow__(2, MyInt(5)))
show("pow(2, MyInt(5), MyInt(3))", lambda: pow(2, MyInt(5), MyInt(3)))
show("pow(MyInt(3), -1, MyInt(7))", lambda: pow(MyInt(3), -1, MyInt(7)))

# complex slots take an int, bool, float or complex subclass operand.
show("complex.__add__(1j, MyComplex(1, 1))", lambda: complex.__add__(1j, MyComplex(1, 1)))
show("complex.__mul__(1j, MyInt(2))", lambda: complex.__mul__(1j, MyInt(2)))
show("complex.__sub__(3j, MyFloat(2.0))", lambda: complex.__sub__(3j, MyFloat(2.0)))
show("complex.__truediv__(4j, MyInt(2))", lambda: complex.__truediv__(4j, MyInt(2)))

# Operator-level delegation reaches the same slot with a subclass on both sides.
show("MyFloat(1.0) + MyFloat(2.0)", lambda: MyFloat(1.0) + MyFloat(2.0))
show("MyFloat(6.0) // MyInt(2)", lambda: MyFloat(6.0) // MyInt(2))
show("MyInt(6) + MyInt(2)", lambda: MyInt(6) + MyInt(2))

# An operand outside the slot's domain still declines with NotImplemented.
show("float.__add__(6.0, 1j)", lambda: float.__add__(6.0, 1j))
show("int.__add__(6, 1.5)", lambda: int.__add__(6, 1.5))
show("int.__add__(6, MyFloat(2.0))", lambda: int.__add__(6, MyFloat(2.0)))


# A subclass that overrides the slot keeps its own dispatch through 3-arg pow.
class PowInt(int):
    def __pow__(self, exp, mod=None):
        return "PowInt.pow"


show("pow(PowInt(2), 5, 3)", lambda: pow(PowInt(2), 5, 3))
