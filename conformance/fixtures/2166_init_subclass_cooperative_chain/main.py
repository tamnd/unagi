class Base:
    registry = []

    def __init_subclass__(cls, /, label=None, **kwargs):
        super().__init_subclass__(**kwargs)
        Base.registry.append((cls.__name__, label))


class Mixin:
    def __init_subclass__(cls, /, tag=None, **kwargs):
        super().__init_subclass__(**kwargs)
        cls.tag = tag


class A(Mixin, Base, label="a", tag="t1"):
    pass


class B(Mixin, Base, label="b", tag="t2"):
    pass


print(Base.registry)
print(A.tag, B.tag)

try:

    class Bad(Base, label="x", bogus=1):
        pass

except TypeError as e:
    print(type(e).__name__ + ": " + str(e))
