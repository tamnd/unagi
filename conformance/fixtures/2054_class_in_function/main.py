# A class defined inside a function body, the shape csv.Sniffer.sniff uses when
# it builds a one-off Dialect subclass per call. The class subclasses a
# module-level base, carries only class variables, gets attributes set on the
# class object afterwards, and is returned. It captures nothing from the
# enclosing scope, the simplest function-local class.


class Base:
    kind = "base"

    def greet(self):
        return "base"


def make(tag):
    class Dialect(Base):
        _name = "sniffed"
        level = 3

    Dialect.tag = tag
    return Dialect


D = make("x")
print(D.__name__, D.__qualname__, D.__module__)
print(D._name, D.level, D.tag, D.kind)
print(D().greet())
print(issubclass(D, Base), isinstance(D(), Base))


# Each call builds a distinct class, so the attribute set on one does not leak
# to another.
D2 = make("y")
print(D.tag, D2.tag, D is D2)
