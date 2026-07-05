class Meta(type):
    def __new__(mcs, name, bases, ns):
        print("new", name)
        ns = dict(ns)
        ns["tag"] = "made"
        return super().__new__(mcs, name, bases, ns)

    def __init__(cls, name, bases, ns):
        print("init", name)
        super().__init__(name, bases, ns)


# Direct three-argument call builds a class through the metaclass protocol.
C = Meta("C", (), {"x": 1})
print(type(C).__name__)
print(C.__name__)
print(C.x)
print(C.tag)
print(isinstance(C, Meta))


class Base:
    pass


D = Meta("D", (Base,), {"y": 2})
print(D.__bases__[0].__name__)
print(D.y)
i = D()
print(type(i).__name__)
