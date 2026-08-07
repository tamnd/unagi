# A complex subclass that overrides an arithmetic dunder runs its own method
# instead of the native complex fast path, on both operand orders and when the
# other operand is itself complex, the way CPython dispatches to the subclass
# slot rather than complex's arithmetic. A subclass that inherits the base
# arithmetic keeps returning a plain complex.


def show(label, fn):
    try:
        v = fn()
        print(label, "=>", type(v).__name__, repr(v))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


class MyComplex(complex):
    def __add__(self, o):
        return type(self)(super().__add__(o))

    __radd__ = __add__

    def __sub__(self, o):
        return type(self)(super().__sub__(o))

    def __mul__(self, o):
        return type(self)(super().__mul__(o))

    __rmul__ = __mul__

    def __truediv__(self, o):
        return type(self)(super().__truediv__(o))

    def __pow__(self, o):
        return type(self)(super().__pow__(o))


z = MyComplex(1, 2)

# Forward slot with an int operand, the path that already worked.
show("MyComplex(1+2j) + 1", lambda: z + 1)
show("MyComplex(1+2j) * 2", lambda: z * 2)

# Forward slot with a complex operand, the path the fast path used to steal.
show("MyComplex(1+2j) - 1j", lambda: z - 1j)
show("MyComplex(1+2j) + 2j", lambda: z + 2j)
show("MyComplex(1+2j) * 1j", lambda: z * 1j)
show("MyComplex(1+2j) / 2j", lambda: z / 2j)
show("MyComplex(1+2j) ** 2", lambda: z ** 2)

# Reflected slot: a plain complex or a real on the left defers to the subclass.
show("1j + MyComplex(1+2j)", lambda: 1j + z)
show("3 * MyComplex(1+2j)", lambda: 3 * z)

# A sentinel override proves the method runs rather than the native arithmetic.
class Tagged(complex):
    def __mul__(self, o):
        return "tagged-mul"

    def __sub__(self, o):
        return "tagged-sub"


t = Tagged(2, 1)
show("Tagged * 4j", lambda: t * 4j)
show("Tagged - 1j", lambda: t - 1j)


# A subclass that inherits the base arithmetic keeps the native path and returns
# a plain complex, unchanged by the guard.
class Plain(complex):
    pass


p = Plain(1, 2)
show("Plain(1+2j) - 1j", lambda: p - 1j)
show("Plain(1+2j) * 2j", lambda: p * 2j)
