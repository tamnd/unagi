import math

class R:
    def __init__(self, v): self.v = v
    def __round__(self, n=None):
        if n is None:
            return ("roundNone", self.v)
        return ("round", self.v, n)
    def __floor__(self): return ("floor", self.v)
    def __ceil__(self): return ("ceil", self.v)
    def __trunc__(self): return ("trunc", self.v)

r = R(5)
print(round(r))
print(round(r, 2))
print(math.floor(r))
print(math.ceil(r))
print(math.trunc(r))

class Bare:
    pass
try:
    round(Bare())
except TypeError as e:
    print("TypeError:", e)
try:
    math.floor(Bare())
except TypeError as e:
    print("TypeError:", e)
