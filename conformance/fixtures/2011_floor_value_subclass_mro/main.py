# A value subclass records its builtin base (tuple, str, int) as a layout rather
# than a class object, so the base was missing from __mro__, __bases__, and
# __base__. CPython lists it between the class and object, which io.py relies on
# when class IOBase(_io._IOBase, metaclass=abc.ABCMeta) puts the C base on the
# linearization the ABC machinery walks.
class T(tuple):
    pass


class S(str):
    pass


class I(int):
    pass


# A bare value subclass names the builtin as its direct base.
print(T.__mro__)
print(T.__bases__)
print(T.__base__)
print(tuple in T.__mro__)
print(S.__mro__)
print(I.__base__)


# A further subclass inherits the layout through the user base: the builtin
# stays on the linearization but is not a direct base.
class U(T):
    pass


print(U.__mro__)
print(U.__bases__)
print(U.__base__)


# A plain user hierarchy is unchanged, and object stays the root with no bases.
class A:
    pass


class B(A):
    pass


print(B.__mro__)
print(B.__bases__)
print(object.__bases__)
print(object.__base__)


# The instance still behaves as its builtin, and isinstance reads the layout.
t = T([1, 2, 3])
print(len(t), t[0], isinstance(t, tuple))
