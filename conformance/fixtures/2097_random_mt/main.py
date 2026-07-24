import _random
import random

# Direct engine: the exact CPython 3.14 Mersenne Twister stream.
r = _random.Random()
r.seed(12345)
print([r.random() for _ in range(3)])
r.seed(12345)
print([r.getrandbits(32) for _ in range(3)])
r.seed(12345)
print(r.getrandbits(100))
r.seed(0)
print(r.random())
r.seed(2**80 + 7)
print(r.random())

# abs(): negative and positive seeds agree.
r.seed(-42)
a = r.random()
r.seed(42)
b = r.random()
print(a == b)

# getstate head and round-trip.
r.seed(12345)
s = r.getstate()
print(len(s), s[-1], s[0], s[1], s[2])
r.seed(999)
r.random()
saved = r.getstate()
v1 = r.random()
r.setstate(saved)
v2 = r.random()
print(v1 == v2)

# getrandbits edge widths.
r.seed(2024)
print(r.getrandbits(1), r.getrandbits(7), r.getrandbits(32), r.getrandbits(64))
print(r.getrandbits(0))

# The random module, driven by the same engine through random.py.
random.seed(42)
print(random.random())
print(random.getrandbits(53))
print(random.randint(1, 100))
print(random.randrange(1000))
print(random.randrange(10, 200, 5))

random.seed(7)
colors = ["red", "green", "blue", "yellow", "purple"]
print(random.choice(colors))
deck = list(range(10))
random.shuffle(deck)
print(deck)
print(random.sample(range(20), 5))

# Same seed reproduces the same sequence.
random.seed(2026)
first = [random.random() for _ in range(5)]
random.seed(2026)
second = [random.random() for _ in range(5)]
print(first == second)

# random.Random is a real subclass of _random.Random, usable directly.
print(issubclass(random.Random, _random.Random))
inst = random.Random(123)
print(inst.random())
print(inst.randint(1, 6))
print(isinstance(inst, _random.Random))
