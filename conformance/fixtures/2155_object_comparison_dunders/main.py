# object's comparison dunders are readable and borrowable as attributes.
# object.__eq__ is identity-or-NotImplemented, object.__ne__ delegates to the
# object's own __eq__ and inverts it, and the four ordering slots always decline.


class Morsel:
    def __init__(self, v):
        self.v = v

    def __eq__(self, other):
        return isinstance(other, Morsel) and self.v == other.v

    # Borrow object's default negation, the shape http.cookies.Morsel uses.
    __ne__ = object.__ne__


a, b, c = Morsel(1), Morsel(1), Morsel(2)
print(a == b, a != b, a == c, a != c)

# The borrowed __ne__ inverts the class's own __eq__.
print(a.__ne__(b), a.__ne__(c))

# object's slots called directly.
print(object.__eq__(a, a), object.__eq__(a, b) is NotImplemented)
print(object.__ne__(a, b))
print(object.__lt__(a, b) is NotImplemented)

# The ordering dunders exist on object and every class inherits them.
print(hasattr(object, "__lt__"), hasattr(object, "__ge__"))
print(hasattr(Morsel, "__le__"), hasattr(Morsel, "__gt__"))

# A class that overrides nothing inherits object's exact slot objects.
print(Morsel.__ne__ is object.__ne__)


class Plain:
    pass


print(Plain.__eq__ is object.__eq__)
print(Plain.__lt__ is object.__lt__)

p, q = Plain(), Plain()
print(p == p, p == q, p != q)
print(p.__eq__(p), p.__eq__(q) is NotImplemented)
print(p.__ne__(q) is NotImplemented)

# Bare objects have no order: both operands decline and the compare raises.
try:
    p < q
except TypeError as e:
    print("TypeError", "not supported" in str(e))


# Reflected dispatch still runs: a subclass refining __eq__ answers first.
class Base:
    def __eq__(self, other):
        return "base"


class Sub(Base):
    def __eq__(self, other):
        return "sub"


print(Base() == Sub())
