import math
import cmath
import copy
import pickle


class Celsius(float):
    pass


class Weighted(complex):
    pass


class Doubled(float):
    def __new__(cls, v):
        return super().__new__(cls, v * 2)


# float subclass: arithmetic falls back to plain float, comparisons and hash
# read the stored value.
t = Celsius(36.6)
print(t + 1, t * 2, t - 0.6, t / 2, t ** 2, -t, abs(Celsius(-3.5)))
print(t == 36.6, t < 40, t > 30, Celsius(1.0) == 1, hash(t) == hash(36.6))
print(repr(t), str(t), format(t, ".2f"), f"{t:08.3f}")
print(isinstance(t, float), isinstance(t, Celsius), type(t + 1).__name__)

# inherited float methods and data attributes.
print(t.is_integer(), Celsius(4.0).is_integer(), Celsius(2.5).hex())
print(Celsius(2.5).as_integer_ratio(), t.real, t.imag, t.conjugate())
print(Celsius(2.5).__getnewargs__())

# math coercion reads the stored double.
print(math.sqrt(Celsius(16.0)), math.floor(Celsius(3.7)), round(Celsius(2.675), 2))
print(math.isclose(Celsius(1.0), 1.0), math.trunc(Celsius(-2.9)))

# __new__ override transforms the stored value.
print(Doubled(3.0), isinstance(Doubled(3.0), float), float(Doubled(3.0)))

# complex subclass: parts, arithmetic, coercion through cmath and complex().
z = Weighted(3, 4)
print(z + 1, z * 2, abs(z), z.conjugate(), z.real, z.imag)
print(z == complex(3, 4), repr(z), complex(z), type(z + 1).__name__)
print(cmath.phase(z), cmath.polar(z), cmath.sqrt(Weighted(-1, 0)))
print(Weighted(2, 3).__getnewargs__())

# dict keys and sets treat a subclass by its value.
d = {Celsius(1.0): "a", 2.0: "b"}
print(d[1.0], len({Celsius(1.0), 1.0}))
print(sorted([Celsius(3), Celsius(1), Celsius(2), 0.5]))

# copy keeps the subclass type and its value.
print(copy.copy(t), copy.deepcopy(t), type(copy.copy(t)).__name__)
print(copy.copy(z), copy.deepcopy(z), type(copy.copy(z)).__name__)

# pickle round-trips across every binary protocol.
for proto in range(2, 6):
    rt = pickle.loads(pickle.dumps(t, proto))
    rz = pickle.loads(pickle.dumps(z, proto))
    print(proto, rt, type(rt).__name__, rt == t, rz, type(rz).__name__, rz == z)
