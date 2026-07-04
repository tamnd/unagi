# The object builtin: the root type, its instances, and the ordering
# constraint it puts on the C3 method resolution order.

# object is a class value that reprs like any other type.
print(object)
print(object.__name__, object.__qualname__)
print(object.__bases__, object.__base__)
print(type(object), type(object()))

# It is the root of every isinstance and issubclass question.
print(isinstance(5, object), isinstance("x", object), isinstance(None, object))
print(isinstance(object, object), isinstance(object, type))
print(issubclass(int, object), issubclass(bool, object), issubclass(object, object))
print(issubclass(object, int))

# A bare object() instance reprs with the address elided so the run is stable.
o = object()
print(repr(o).startswith("<object object at 0x"))
print(isinstance(o, object), type(o) is object)

# object() takes no arguments.
try:
    object(1)
except TypeError as e:
    print("obj-arg", e)


# A class deriving from object explicitly behaves like the implicit form.
class C(object):
    def __init__(self, x):
        self.x = x


c = C(3)
print(c.x, isinstance(c, object), issubclass(C, object))
print(C.__base__)
print([t.__name__ for t in C.__mro__])


# object may sit at the end of a multiple-base list.
class A:
    pass


class B(A):
    pass


class E(B, object):
    pass


print([t.__name__ for t in E.__mro__])


# Listing object before another base is an inconsistent order.
try:
    class Z(object, B):
        pass
except TypeError as e:
    print("mro", e)


# object() as a class pattern matches every value.
def tagged(v):
    match v:
        case object():
            return "obj"
    return "no"


print(tagged(5), tagged(None), tagged([1]), tagged(C(0)))
