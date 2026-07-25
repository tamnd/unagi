# x.__class__ = C rebinds an instance's type in place when the two classes give
# their instances the same layout (same __dict__/__weakref__ support and the
# same slot names), and raises the CPython TypeError otherwise. Deleting
# __class__ is refused, and the target must be a class.


class A:
    def who(self):
        return "A"


class B:
    def who(self):
        return "B"


a = A()
a.__class__ = B
print(type(a).__name__, a.who())


# A subclass that adds no slots keeps A's layout, so the rebind is allowed.
class Sub(A):
    def extra(self):
        return "sub"


a2 = A()
a2.__class__ = Sub
print(type(a2).__name__, a2.extra())


# Incompatible: a different number of slots.
class S1:
    __slots__ = ('x',)


class S2:
    __slots__ = ('x', 'y')


s = S1()
try:
    s.__class__ = S2
    print("S1->S2 no error")
except TypeError as e:
    print("S1->S2:", e)


# Incompatible: same count, different slot name.
class U2:
    __slots__ = ('z',)


try:
    S1().__class__ = U2
    print("S1->U2 no error")
except TypeError as e:
    print("S1->U2:", e)


# Incompatible: __dict__ layout versus slots layout.
class P:
    pass


class Q:
    __slots__ = ('x',)


try:
    P().__class__ = Q
    print("P->Q no error")
except TypeError as e:
    print("P->Q:", e)


# Compatible: identical slot names, slot value preserved across the rebind.
class T1:
    __slots__ = ('x',)


class T2:
    __slots__ = ('x',)


t = T1()
t.x = 7
t.__class__ = T2
print("T1->T2", type(t).__name__, t.x)


# The target must be a class, and __class__ cannot be deleted.
try:
    a.__class__ = 5
except TypeError as e:
    print("nontype:", e)

try:
    del a.__class__
except TypeError as e:
    print("del:", e)
