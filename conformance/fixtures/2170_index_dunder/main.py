class I:
    def __init__(self, n):
        self.n = n
    def __index__(self):
        return self.n

xs = [10, 20, 30, 40, 50]
print(list(range(I(3))))
print(list(range(I(1), I(4))))
print(list(range(I(0), I(10), I(3))))
print(xs[I(2)])
print(xs[I(-1)])
print(bin(I(5)), hex(I(255)), oct(I(8)))
print(chr(I(65)))
print("hello"[I(1)])
print(xs[I(1):I(4)])
print(xs[I(1):I(4):I(2)])
xs[I(0)] = 99
print(xs)
del xs[I(0)]
print(xs)

class Bad:
    def __index__(self):
        return "x"

try:
    xs[Bad()]
except TypeError as e:
    print("TypeError:", e)
try:
    range(Bad())
except TypeError as e:
    print("TypeError:", e)
