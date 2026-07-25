# An iterator answers __next__ and __iter__ the way CPython's does: __next__
# advances the cursor and raises StopIteration once it is spent, and __iter__
# returns the iterator itself so iter(it) is it. Code that drives an iterator by
# hand relies on both, and inspect reaches iter(lines).__next__ this way.
seq = [10, 20, 30]
it = iter(seq)

# iter() is idempotent on an iterator: iter(it) hands back the same object.
print(iter(it) is it)
print(it.__iter__() is it)

# __next__ walks the elements, then StopIteration ends it.
print(it.__next__())
print(it.__next__())
print(it.__next__())
try:
    it.__next__()
except StopIteration:
    print("stopped")

# The next() builtin and the __next__ read agree, and both bind as callables.
step = iter(seq).__next__
print(step())
print(step())

# A generator is its own iterator and answers the same two dunders.
def count():
    yield "a"
    yield "b"

gen = count()
print(iter(gen) is gen)
print(gen.__next__())
print(gen.__next__())
try:
    gen.__next__()
except StopIteration:
    print("gen stopped")

# The lazy map, filter and zip shapes are iterators too.
print(map(str, [1, 2]).__next__())
print(filter(None, [0, 3]).__next__())
print(enumerate(["z"]).__next__())
print(zip([4], [5]).__next__())
