# A property is a data descriptor, so it answers __get__, __set__ and __delete__.
# enum's _is_descriptor probes hasattr(obj, '__get__') to tell a member value
# from a descriptor, so a property that did not answer the protocol was
# misfiled as a member. That broke `import http`, whose HTTPStatus mixes @property
# helpers with tuple members.


def is_descriptor(obj):
    return (
        hasattr(obj, "__get__")
        or hasattr(obj, "__set__")
        or hasattr(obj, "__delete__")
    )


class C:
    def __init__(self):
        self._x = 1

    @property
    def x(self):
        return self._x

    @x.setter
    def x(self, v):
        self._x = v

    @x.deleter
    def x(self):
        self._x = None


p = C.__dict__["x"]

# The descriptor protocol dunders are all present and callable.
print(hasattr(p, "__get__"), hasattr(p, "__set__"), hasattr(p, "__delete__"))
print(is_descriptor(p))
print(is_descriptor((1, 2, 3)))

c = C()
# __get__ with an instance calls the getter; with None it returns the property.
print(p.__get__(c, C))
print(p.__get__(None, C) is p)

# __set__ and __delete__ drive the setter and deleter.
p.__set__(c, 42)
print(c._x)
p.__delete__(c)
print(c._x)


# A read-only property raises the no-setter and no-deleter errors.
class R:
    @property
    def y(self):
        return 5


ry = R.__dict__["y"]
try:
    ry.__set__(R(), 9)
except AttributeError:
    print("no setter")
try:
    ry.__delete__(R())
except AttributeError:
    print("no deleter")

# The whole reason: a class mixing property helpers and tuple members classifies
# each correctly, so http imports.
import http

print(http.HTTPStatus.OK, http.HTTPStatus.OK.phrase)
print(http.HTTPStatus.NOT_FOUND.value, http.HTTPStatus.NOT_FOUND.is_client_error)
