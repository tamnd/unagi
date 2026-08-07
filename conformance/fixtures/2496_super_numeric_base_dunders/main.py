# A numeric subclass reaches its base arithmetic through super(), so a class that
# overrides an operator dunder and delegates upward runs the base int, float or
# complex arithmetic on its stored value and can rebuild its own type.


def show(label, fn):
    try:
        v = fn()
        print(label, "=>", type(v).__name__, repr(v))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


class MyInt(int):
    def __add__(self, o):
        return type(self)(super().__add__(o))

    __radd__ = __add__

    def __sub__(self, o):
        return type(self)(super().__sub__(o))

    def __mul__(self, o):
        return type(self)(super().__mul__(o))

    def __floordiv__(self, o):
        return type(self)(super().__floordiv__(o))

    def __mod__(self, o):
        return type(self)(super().__mod__(o))

    def __pow__(self, o):
        return type(self)(super().__pow__(o))

    def __and__(self, o):
        return type(self)(super().__and__(o))

    def __or__(self, o):
        return type(self)(super().__or__(o))

    def __xor__(self, o):
        return type(self)(super().__xor__(o))

    def __lshift__(self, o):
        return type(self)(super().__lshift__(o))

    def __rshift__(self, o):
        return type(self)(super().__rshift__(o))


a = MyInt(6)
show("MyInt(6) + 2", lambda: a + 2)
show("2 + MyInt(6)", lambda: 2 + a)
show("MyInt(6) - 2", lambda: a - 2)
show("MyInt(6) * 3", lambda: a * 3)
show("MyInt(7) // 2", lambda: MyInt(7) // 2)
show("MyInt(7) % 3", lambda: MyInt(7) % 3)
show("MyInt(2) ** 5", lambda: MyInt(2) ** 5)
show("MyInt(6) & 3", lambda: a & 3)
show("MyInt(6) | 1", lambda: a | 1)
show("MyInt(6) ^ 3", lambda: a ^ 3)
show("MyInt(1) << 4", lambda: MyInt(1) << 4)
show("MyInt(64) >> 2", lambda: MyInt(64) >> 2)
show("MyInt(255).bit_length()", lambda: MyInt(255).bit_length())


class MyFloat(float):
    def __truediv__(self, o):
        return type(self)(super().__truediv__(o))

    def __rtruediv__(self, o):
        return type(self)(super().__rtruediv__(o))

    def __add__(self, o):
        return type(self)(super().__add__(o))

    __radd__ = __add__

    def __mul__(self, o):
        return type(self)(super().__mul__(o))

    __rmul__ = __mul__

    def __pow__(self, o):
        return type(self)(super().__pow__(o))

    def __floordiv__(self, o):
        return type(self)(super().__floordiv__(o))

    def __mod__(self, o):
        return type(self)(super().__mod__(o))


f = MyFloat(6.0)
show("MyFloat(6.0) / 2", lambda: f / 2)
show("1 / MyFloat(2.0)", lambda: 1 / MyFloat(2.0))
show("MyFloat(1.0) + 2.0", lambda: MyFloat(1.0) + 2.0)
show("3 * MyFloat(2.0)", lambda: 3 * MyFloat(2.0))
show("MyFloat(2.0) ** 3", lambda: MyFloat(2.0) ** 3)
show("MyFloat(7.0) // 2", lambda: MyFloat(7.0) // 2)
show("MyFloat(7.5) % 2", lambda: MyFloat(7.5) % 2)
show("MyFloat(2.5).is_integer()", lambda: MyFloat(2.5).is_integer())
show("MyFloat(4.0).is_integer()", lambda: MyFloat(4.0).is_integer())


class MyComplex(complex):
    def __add__(self, o):
        return type(self)(super().__add__(o))

    def __mul__(self, o):
        return type(self)(super().__mul__(o))


z = MyComplex(1, 2)
show("MyComplex(1+2j) + 1", lambda: z + 1)
show("MyComplex(1+2j) * 2", lambda: z * 2)


# A super() delegation reaches the object-default __init__ as a no-op, so a
# numeric subclass with its own __init__ still builds cleanly.
class Tagged(float):
    def __init__(self, v):
        super().__init__()
        self.tag = "t"


tg = Tagged(3.5)
show("Tagged(3.5) built", lambda: (type(tg).__name__, float(tg), tg.tag))


# The statistics type-conservation shape: harmonic mean keeps the subclass type
# because its reciprocal and sum stay MyFloat through the super() arithmetic.
import statistics

data = [MyFloat(x) for x in (1, 2, 4)]
show("harmonic_mean keeps subclass", lambda: type(statistics.harmonic_mean(data)).__name__)
show("mean keeps subclass", lambda: type(statistics.mean(data)).__name__)
