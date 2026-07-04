# del o.attr removes an instance attribute, and reading it afterward is the same
# miss a never-set name gives. A data descriptor with __delete__ intercepts the
# delete, a property runs its deleter (or raises when it has none), and deleting
# a plain missing attribute is an AttributeError. On a class del removes the
# class-dict entry.
class Box:
    def __init__(self):
        self.a = 1


b = Box()
del b.a
try:
    b.a
except AttributeError as e:
    print(e)


class Trace:
    def __get__(self, obj, owner):
        return "value"

    def __set__(self, obj, val):
        pass

    def __delete__(self, obj):
        print("delete called")


class C:
    f = Trace()


c = C()
del c.f


class P:
    @property
    def x(self):
        return 1

    @x.deleter
    def x(self):
        print("deleter ran")


del P().x


class NoDel:
    @property
    def y(self):
        return 1


try:
    del NoDel().y
except AttributeError as e:
    print(e)


class Bare:
    pass


try:
    del Bare().nope
except AttributeError as e:
    print(e)


class K:
    z = 5


del K.z
try:
    K.z
except AttributeError as e:
    print(e)
