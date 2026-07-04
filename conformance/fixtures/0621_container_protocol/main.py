# User classes plug into the container and callable protocol: __len__ backs
# len(), __getitem__/__setitem__/__delitem__ back subscription, __iter__ with
# __next__ (or __getitem__ alone) backs iteration and membership, __contains__
# short-circuits `in`, and __call__ makes an instance callable.


class Box:
    def __init__(self, items):
        self.items = items

    def __len__(self):
        return len(self.items)

    def __getitem__(self, i):
        return self.items[i]

    def __setitem__(self, i, v):
        self.items[i] = v

    def __delitem__(self, i):
        del self.items[i]

    def __contains__(self, x):
        return x in self.items

    def __call__(self, extra):
        return len(self.items) + extra


b = Box([10, 20, 30])
print(len(b))
print(b[0], b[2])
b[1] = 99
print(b[1])
del b[0]
print(b.items)
print(20 in b, 99 in b)
print(b(5))


class Count:
    def __init__(self, n):
        self.n = n
        self.i = 0

    def __iter__(self):
        return self

    def __next__(self):
        if self.i >= self.n:
            raise StopIteration
        self.i += 1
        return self.i


print([x for x in Count(3)])
total = 0
for v in Count(4):
    total += v
print(total)
print(list(Count(2)))

c = Count(2)
print(next(c), next(c), next(c, "done"))


class Seq:
    def __getitem__(self, i):
        if i > 2:
            raise IndexError
        return i * 10


print([x for x in Seq()])
print(20 in Seq(), 5 in Seq())


class Bare:
    pass


def show(fn):
    try:
        fn()
        print("no error")
    except TypeError as e:
        print("TypeError: " + str(e))
    except ValueError as e:
        print("ValueError: " + str(e))


bare = Bare()
show(lambda: len(bare))
show(lambda: bare[0])
show(lambda: bare())


class BadLen:
    def __len__(self):
        return -1


class StrLen:
    def __len__(self):
        return "x"


show(lambda: len(BadLen()))
show(lambda: len(StrLen()))
