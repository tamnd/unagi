# dataclasses builds __init__, __repr__, __eq__ and friends from the annotated
# fields of a class. This walks the common surface: defaults and default_factory,
# the generated repr and eq, ordering, frozen instances, field metadata, asdict
# and astuple, replace, InitVar-free post_init, and inheritance of fields.
from dataclasses import (
    dataclass, field, fields, asdict, astuple, replace, is_dataclass, FrozenInstanceError,
)

@dataclass
class Point:
    x: int
    y: int = 0

p = Point(1)
print("repr:", p)
print("eq:", Point(1, 0) == Point(1, 0), Point(1) == Point(2))
print("is_dataclass:", is_dataclass(Point), is_dataclass(p))
print("fields:", [f.name for f in fields(p)])

# default_factory seeds a fresh mutable per instance.
@dataclass
class Bag:
    items: list = field(default_factory=list)

b1 = Bag()
b2 = Bag()
b1.items.append(1)
print("factory independent:", b1.items, b2.items)

# order=True generates the comparison operators.
@dataclass(order=True)
class Ver:
    major: int
    minor: int

print("order:", Ver(1, 2) < Ver(1, 3), Ver(2, 0) > Ver(1, 9))
print("sorted:", sorted([Ver(2, 0), Ver(1, 5), Ver(1, 2)]))

# frozen=True blocks attribute assignment.
@dataclass(frozen=True)
class Const:
    name: str
    value: int

c = Const("pi", 3)
print("frozen repr:", c)
try:
    c.value = 4
except FrozenInstanceError:
    print("frozen blocks assignment")

# asdict and astuple recurse into nested dataclasses.
@dataclass
class Line:
    start: Point
    end: Point

ln = Line(Point(0, 0), Point(1, 1))
print("asdict:", asdict(ln))
print("astuple:", astuple(ln))

# replace makes a modified copy.
p2 = replace(p, y=9)
print("replace:", p2, p)

# field metadata and repr=False.
@dataclass
class Meta:
    a: int = field(metadata={"unit": "m"})
    secret: int = field(default=0, repr=False)

m = Meta(5)
print("repr hides field:", m)
print("metadata:", fields(m)[0].metadata["unit"])

# __post_init__ runs after the generated __init__.
@dataclass
class Celsius:
    temp: float
    fahrenheit: float = 0.0
    def __post_init__(self):
        self.fahrenheit = self.temp * 9 / 5 + 32

print("post_init:", Celsius(100).fahrenheit)

# inheritance stacks fields base-first.
@dataclass
class Base:
    a: int

@dataclass
class Derived(Base):
    b: int = 2

d = Derived(1)
print("inherited fields:", [f.name for f in fields(d)], d)
