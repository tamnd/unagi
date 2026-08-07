# A float subclass that overrides arithmetic dunders must dispatch its own
# methods, in both operand orders, rather than the native float fast path.


def show(label, fn):
    try:
        v = fn()
        print(label, "=>", type(v).__name__, repr(v))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


class Sentinel(float):
    def __add__(self, other):
        return ("add", other)

    def __radd__(self, other):
        return ("radd", other)

    def __sub__(self, other):
        return ("sub", other)

    def __mul__(self, other):
        return ("mul", other)

    def __rmul__(self, other):
        return ("rmul", other)

    def __truediv__(self, other):
        return ("truediv", other)

    def __rtruediv__(self, other):
        return ("rtruediv", other)

    def __floordiv__(self, other):
        return ("floordiv", other)

    def __mod__(self, other):
        return ("mod", other)

    def __pow__(self, other):
        return ("pow", other)


s = Sentinel(2.0)
show("s + 3", lambda: s + 3)
show("3 + s", lambda: 3 + s)
show("3.0 + s", lambda: 3.0 + s)
show("s - 1", lambda: s - 1)
show("s * 4", lambda: s * 4)
show("4 * s", lambda: 4 * s)
show("4.0 * s", lambda: 4.0 * s)
show("s / 2", lambda: s / 2)
show("2 / s", lambda: 2 / s)
show("s // 2", lambda: s // 2)
show("s % 2", lambda: s % 2)
show("s ** 3", lambda: s ** 3)


# A subclass that keeps its type through the operation, the shape statistics
# relies on for type conservation.
class Typed(float):
    def __truediv__(self, other):
        return type(self)(float(self) / float(other))

    def __rtruediv__(self, other):
        return type(self)(float(other) / float(self))

    def __add__(self, other):
        return type(self)(float(self) + float(other))

    __radd__ = __add__

    def __mul__(self, other):
        return type(self)(float(self) * float(other))

    __rmul__ = __mul__


t = Typed(6.0)
show("Typed(6)/2", lambda: t / 2)
show("1/Typed(6)", lambda: 1 / t)
show("Typed(6)+Typed(4)", lambda: t + Typed(4.0))
show("2*Typed(6)", lambda: 2 * t)
show("sum([Typed])", lambda: sum([Typed(1.0), Typed(2.0), Typed(3.0)]))


# A plain subclass with no overrides still collapses to a base float, the way
# CPython drops the subclass when it inherits float's arithmetic.
class Plain(float):
    pass


p = Plain(2.5)
show("Plain(2.5)+1", lambda: p + 1)
show("Plain(2.5)*Plain(2)", lambda: p * Plain(2.0))
show("1+Plain(2.5)", lambda: 1 + p)
show("Plain(2.5)/2", lambda: p / 2)
