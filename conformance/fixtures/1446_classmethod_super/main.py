# super() inside a classmethod binds the class it was called on and walks its
# MRO past the defining class, both the zero-argument form and the explicit
# super(C, cls) form. __init_subclass__ chaining relies on the same machinery:
# a base hook calls super().__init_subclass__() which cooperates down to the
# object-root no-op.
class A:
    @classmethod
    def make(cls):
        return "A.make:" + cls.__name__


class B(A):
    @classmethod
    def make(cls):
        return "B->" + super().make()


print(B.make())


class C(B):
    @classmethod
    def make(cls):
        return "C->" + super(C, cls).make()


print(C.make())


class Base:
    registry = []

    def __init_subclass__(cls):
        super().__init_subclass__()
        Base.registry.append(cls.__name__)


class X(Base):
    pass


class Y(Base):
    pass


print(Base.registry)
