class D:
    def __init__(self, v): self.v = v
    def __pow__(self, other, mod=None):
        return ("pow", self.v, other, mod)
    def __rpow__(self, other, mod=None):
        return ("rpow", self.v, other, mod)

print(pow(D(5), 3))
print(pow(D(5), 3, 7))
print(pow(2, D(5), 7))
print(pow(2, 3, 7))

try:
    pow(2.0, 3, 7)
except TypeError as e:
    print("TypeError:", e)

class N:
    pass
try:
    pow(N(), 3, 7)
except TypeError as e:
    print("TypeError:", e)
