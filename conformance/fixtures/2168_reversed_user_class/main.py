class R:
    def __init__(self, data):
        self.data = data

    def __reversed__(self):
        return iter(self.data[::-1])


r = R(["a", "b", "c"])
print(list(reversed(r)))


class S:
    def __init__(self, n):
        self.n = n

    def __len__(self):
        return self.n

    def __getitem__(self, i):
        return i * 10


s = S(3)
it = reversed(s)
print(type(it).__name__)
print(list(it))
print(list(reversed(S(0))))


class Q:
    pass


try:
    reversed(Q())
except TypeError as e:
    print("TypeError:", e)


class BadRev:
    def __reversed__(self):
        return 42


print(reversed(BadRev()))
