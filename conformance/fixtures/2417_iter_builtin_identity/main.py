# The iter() builtin is defined as type(x).__iter__(x): it hands back exactly the
# object __iter__ returned. For an iterator whose __iter__ returns self that means
# iter(x) is x, and the result keeps its own type rather than a generic handle.
# unagi used to wrap every class instance in a generic iterator object, so the
# identity was lost and the type came back as iterator. This pins the contract
# across a self-returning iterator, a generator, an old-style __getitem__
# sequence, re-iteration and the non-iterator error.


class SelfIter:
    def __init__(self):
        self.i = 0

    def __iter__(self):
        return self

    def __next__(self):
        self.i += 1
        if self.i > 3:
            raise StopIteration
        return self.i


class Seq:
    # An old-style sequence has no __iter__, only __getitem__, so iter() builds a
    # fresh sequence iterator that is not the sequence itself.
    def __init__(self, data):
        self.data = data

    def __getitem__(self, i):
        return self.data[i]


def gen():
    yield 1
    yield 2


class BadIter:
    def __iter__(self):
        return 42


# A self-returning iterator: iter() hands back the very object, keeping its type.
s = SelfIter()
print("self identity:", iter(s) is s)
print("self type:", type(iter(s)).__name__)
print("self list:", list(SelfIter()))

# A generator is its own iterator too.
g = gen()
print("gen identity:", iter(g) is g)
print("gen type:", type(iter(g)).__name__)
print("gen list:", list(gen()))

# An old-style sequence iterates through __getitem__ from index zero, and its
# iterator is a distinct object.
seq = Seq([10, 20, 30])
print("seq list:", list(seq))
print("seq iter is not seq:", iter(seq) is not seq)
it = iter(seq)
print("seq iter next:", next(it), next(it))

# iter() over an object that is already an iterator is idempotent.
base = iter(SelfIter())
print("reiter identity:", iter(base) is base)

# A container whose __iter__ delegates to another iterable is not its own
# iterator, and iterating it still yields the delegated items.
class Container:
    def __init__(self, items):
        self.items = items

    def __iter__(self):
        return iter(self.items)


c = Container([7, 8, 9])
print("container is not self:", iter(c) is not c)
print("container list:", list(c))

# An __iter__ that returns a non-iterator raises TypeError from iter() eagerly.
try:
    iter(BadIter())
except TypeError as e:
    print("bad:", str(e))
