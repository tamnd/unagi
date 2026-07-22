# A class defined inside another class body, the shape weakref.py and inspect.py
# lean on. The nested class binds as an attribute of the enclosing class and
# carries its own methods, class variables and further nesting.


class Outer:
    kind = "outer"

    class Inner:
        __slots__ = ("a", "b")
        label = "inner"

        def __init__(self, a, b):
            self.a = a
            self.b = b

        def total(self):
            return self.a + self.b

        class Deep:
            def note(self):
                return "deep"

    def make(self, a, b):
        return Outer.Inner(a, b)


# The nested class reads back as an attribute and constructs normally.
o = Outer()
inner = o.make(3, 4)
print(inner.total())
print(inner.a, inner.b)
print(Outer.kind, Outer.Inner.label)
print(Outer.Inner.__name__, Outer.Inner.__qualname__)
print(Outer.Inner.total.__qualname__)
print(Outer.Inner.Deep.__qualname__)
print(Outer.Inner.Deep().note())
print(isinstance(inner, Outer.Inner))


# A nested class subclassing a top-level class keeps normal MRO behaviour, and a
# nested class with only a class variable and no methods lowers too.
class Base:
    def greet(self):
        return "base"


class Holder:
    tag = 1

    class Child(Base):
        def greet(self):
            return "child of " + super().greet()

    class Config:
        debug = False
        retries = 3


print(Holder.Child().greet())
print(Holder.Config.debug, Holder.Config.retries)
print(Holder.tag)
print(issubclass(Holder.Child, Base))
