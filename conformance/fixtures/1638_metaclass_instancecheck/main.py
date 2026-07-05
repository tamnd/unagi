# A metaclass __instancecheck__/__subclasscheck__ decides isinstance and
# issubclass outright, the way abc.ABCMeta registers virtual subclasses.

class Registry(type):
    def __init__(cls, name, bases, ns):
        super().__init__(name, bases, ns)
        cls._names = list(ns.get("_seed", ()))

    def register(cls, other):
        cls._names.append(other.__name__)
        return other

    def __instancecheck__(cls, obj):
        return type(obj).__name__ in cls._names

    def __subclasscheck__(cls, sub):
        return sub.__name__ in cls._names


class Drawable(metaclass=Registry):
    _seed = ("Circle",)


class Circle:
    pass


class Square:
    pass


@Drawable.register
class Triangle:
    pass


print("isinstance(Circle(), Drawable):", isinstance(Circle(), Drawable))
print("isinstance(Square(), Drawable):", isinstance(Square(), Drawable))
print("isinstance(Triangle(), Drawable):", isinstance(Triangle(), Drawable))

print("issubclass(Circle, Drawable):", issubclass(Circle, Drawable))
print("issubclass(Square, Drawable):", issubclass(Square, Drawable))
print("issubclass(Triangle, Drawable):", issubclass(Triangle, Drawable))

# A real subclass still answers through the hook, not the MRO.
class Sub(Drawable):
    pass

print("isinstance(Sub(), Drawable):", isinstance(Sub(), Drawable))
print("issubclass(Sub, Drawable):", issubclass(Sub, Drawable))

# The tuple form consults each element's hook.
print("isinstance(Circle(), (Drawable, int)):", isinstance(Circle(), (Drawable, int)))
print("isinstance(Square(), (Drawable, str)):", isinstance(Square(), (Drawable, str)))

# A plain class keeps the ordinary structural check.
class Base:
    pass

class Derived(Base):
    pass

print("isinstance(Derived(), Base):", isinstance(Derived(), Base))
print("issubclass(Derived, Base):", issubclass(Derived, Base))

# A raise inside the hook propagates.
class StrictMeta(type):
    def __instancecheck__(cls, obj):
        raise TypeError("no checks here")


class Strict(metaclass=StrictMeta):
    pass


try:
    isinstance(1, Strict)
except TypeError as e:
    print("propagated:", e)
