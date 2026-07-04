# A user object whose type defines __get__ and __set__ is a data descriptor: it
# intercepts both read and write on the owning class's instances, outranking the
# instance dict, and reading it off the class passes a None instance so it can
# hand back itself. An object with only __get__ is a non-data descriptor, so a
# write lands in the instance dict and then shadows it.
class Logged:
    def __init__(self):
        self.value = None

    def __get__(self, obj, owner):
        if obj is None:
            return self
        print("get")
        return self.value

    def __set__(self, obj, value):
        print("set", value)
        self.value = value


class C:
    x = Logged()


c = C()
c.x = 5
print(c.x)
print(isinstance(C.x, Logged))


class Cached:
    def __get__(self, obj, owner):
        if obj is None:
            return self
        return 99


class D:
    y = Cached()


d = D()
print(d.y)
d.y = 7
print(d.y)
