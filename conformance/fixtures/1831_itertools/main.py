# The itertools module, provided in Go behind the itertools import. This
# exercises the infinite iterators bounded with islice, the terminating
# iterators over the input sequences, the combinatoric generators, and the
# groupby, tee, and batched surfaces, all consumed the way a Python program
# would.
import itertools
from itertools import count, cycle, repeat, chain, islice

print(list(islice(count(), 5)))
print(list(islice(count(10, 2), 4)))
print(list(islice(count(2.5, 0.5), 3)))
print(list(islice(cycle([1, 2, 3]), 7)))
print(list(repeat("x", 3)))
print(list(islice(repeat(9), 4)))

print(list(chain([1, 2], [3], [4, 5])))
print(list(chain.from_iterable([[1, 2], [3, 4]])))
print(list(islice(range(10), 2, 8, 2)))
print(list(itertools.compress("ABCDEF", [1, 0, 1, 0, 1, 1])))
print(list(itertools.takewhile(lambda x: x < 5, [1, 4, 6, 4, 1])))
print(list(itertools.dropwhile(lambda x: x < 5, [1, 4, 6, 4, 1])))
print(list(itertools.filterfalse(lambda x: x % 2, range(10))))
print(list(itertools.starmap(pow, [(2, 5), (3, 2), (10, 3)])))

print(list(itertools.accumulate([1, 2, 3, 4, 5])))
print(list(itertools.accumulate([1, 2, 3, 4], initial=100)))
print(list(itertools.accumulate([1, 2, 3, 4], lambda a, b: a * b)))
print(list(itertools.pairwise([1, 2, 3, 4])))
print(list(itertools.zip_longest([1, 2, 3], [4, 5], fillvalue=0)))

print(list(itertools.product([1, 2], [3, 4])))
print(list(itertools.product([0, 1], repeat=3)))
print(list(itertools.permutations([1, 2, 3])))
print(list(itertools.permutations([1, 2, 3], 2)))
print(list(itertools.combinations([1, 2, 3, 4], 2)))
print(list(itertools.combinations_with_replacement([1, 2, 3], 2)))

print(list(itertools.batched(range(5), 2)))
print([(k, list(g)) for k, g in itertools.groupby([1, 1, 2, 3, 3, 3, 1])])
print([k for k, g in itertools.groupby("aAbBcCdd", key=lambda c: c.lower())])

a, b = itertools.tee([10, 20, 30])
print(list(a), list(b))


def batches_strict(seq, n):
    try:
        return list(itertools.batched(seq, n, strict=True))
    except ValueError as e:
        return str(e)


print(batches_strict([1, 2, 3, 4], 2))
print(batches_strict([1, 2, 3, 4, 5], 2))
