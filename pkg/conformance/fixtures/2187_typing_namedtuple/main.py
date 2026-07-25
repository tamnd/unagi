# typing.NamedTuple runs unmodified: its metaclass reads the class body's PEP 649
# __annotate__ off the namespace, builds a collections.namedtuple, and hangs the
# annotations, user methods, defaults, properties and staticmethods on it. The
# generated type behaves as a real class for construction, isinstance, and the
# namedtuple helpers.
from typing import NamedTuple, get_type_hints


class Employee(NamedTuple):
    name: str
    id: int = 3
    dept: str = "eng"

    def greet(self):
        return "hi " + self.name

    @property
    def label(self):
        return self.name + "/" + self.dept

    @staticmethod
    def kind():
        return "person"


e = Employee("Guido")
print(e)
print(e.name, e.id, e.dept)
print(e.greet(), e.label, e.kind())
print(e._asdict())
print(e._replace(id=7))
print(Employee._fields, Employee._field_defaults)
print(list(e), e[0], len(e))
a, b, c = e
print(a, b, c)
print(e == Employee("Guido", 3, "eng"), hash(e) == hash(Employee("Guido", 3, "eng")))
print(isinstance(e, Employee), isinstance(e, tuple), isinstance((1, 2), Employee))
print(Employee.__annotations__)
print(get_type_hints(Employee))
print({Employee("A"): 1}[Employee("A")])

# functional form
Rec = NamedTuple("Rec", [("a", int), ("b", str)])
r = Rec(1, "hi")
print(r, r.a, r.b, Rec.__annotations__)

# empty NamedTuple
class Empty(NamedTuple):
    pass


print(Empty(), Empty._fields)

# a non-default field after a default one is rejected
try:
    class Bad(NamedTuple):
        x: int = 1
        y: int
except TypeError as exc:
    print("TypeError:", exc)

# a namedtuple special name cannot be overwritten
try:
    class Bad2(NamedTuple):
        x: int
        _fields = 3
except AttributeError as exc:
    print("AttributeError:", exc)
