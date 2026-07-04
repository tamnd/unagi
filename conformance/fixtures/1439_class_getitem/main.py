# A class defining __class_getitem__ makes C[item] a call to that hook, which
# CPython treats as an implicit classmethod so cls comes first. A subclass sees
# its own class, and a class without the hook is not subscriptable.
class Box:
    def __class_getitem__(cls, item):
        return cls.__name__ + "[" + repr(item) + "]"


print(Box[7])
print(Box["x"])
print(Box[(1, 2)])


class Sub(Box):
    pass


print(Sub["s"])


class Explicit:
    @classmethod
    def __class_getitem__(cls, item):
        return "explicit:" + cls.__name__ + ":" + str(item)


print(Explicit[42])


class Plain:
    pass


try:
    Plain[0]
except TypeError as e:
    print(e)
