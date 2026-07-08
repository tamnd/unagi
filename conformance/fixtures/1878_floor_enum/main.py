# enum is compiled from its CPython source in the floor, so a program that uses
# Enum, IntEnum, StrEnum, Flag, IntFlag, auto and the functional API runs the
# same module body the oracle runs.
from enum import Enum, IntEnum, StrEnum, Flag, IntFlag, auto, unique

class Color(Enum):
    RED = 1
    GREEN = 2
    BLUE = 3
    def describe(self):
        return f"{self.name}={self.value}"

print(Color.RED, Color.RED.name, Color.RED.value)
print(list(Color))
print(Color(2))
print(Color["GREEN"])
print(len(Color))
print(Color.RED in Color)
print(Color.BLUE.describe())
print(Color.__members__)

class Size(IntEnum):
    S = 1
    M = 2
    L = 3

print(Size.M + 1, Size.L > Size.S, int(Size.M))

class Kind(StrEnum):
    A = "a"
    B = "b"

print(Kind.A, Kind.A == "a", Kind.A.upper())

class Perm(IntFlag):
    R = 4
    W = 2
    X = 1

p = Perm.R | Perm.W
print(p, int(p))
print(Perm.R in p)
print(list(p))
print(~Perm.R)

class Auto(Enum):
    A = auto()
    B = auto()
    C = auto()

print([m.value for m in Auto])

@unique
class Uniq(Enum):
    ONE = 1
    TWO = 2

print(list(Uniq))

Animal = Enum("Animal", ["CAT", "DOG", "BIRD"])
print(list(Animal), Animal.DOG.value)

try:
    Color(99)
except ValueError as e:
    print("ValueError:", e)
