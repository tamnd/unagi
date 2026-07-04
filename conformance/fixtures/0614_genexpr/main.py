# A generator expression is the one comprehension form that is not inlined: it
# is a real generator, lazy, taking the eagerly-evaluated outermost iterable as
# its source. This fixture pins that shape against CPython 3.14.

# Basic genexp driven by list().
g = (x * x for x in range(5))
print(list(g))

# A genexp is lazy, but the outermost iterable is evaluated eagerly at creation.
def src():
    print("src called")
    return [1, 2, 3]

gen = (n + 1 for n in src())
print("after create")
print(list(gen))

# Multiple for clauses and an if filter, threaded lazily per outer item.
print(list(i + j for i in range(3) for j in range(2) if i != j))

# Feeding container constructors and reducers.
print(sum(v * v for v in range(4)))
print(max(a for a in [3, 1, 2]))
print(sorted(c for c in "dbca"))
print(set(x % 3 for x in range(6)) == {0, 1, 2})

# Iteration variables are isolated: an enclosing name of the same spelling is
# untouched, and each element is computed lazily.
w = 100
squares = (w * w for w in range(3))
print(list(squares), w)

# Tuple clause target.
pairs = [(1, "a"), (2, "b")]
print(list(k for k, _ in pairs))

# A walrus inside a genexp leaks to the enclosing scope (PEP 572).
total = 0
acc = (total := total + n for n in range(4))
print(list(acc), total)

# Late binding: the clause variable is one cell, so lambdas built in the body
# all see its final value once the genexp is fully consumed.
fns = list((lambda: i) for i in range(3))
print([f() for f in fns])

# The outermost iterable failing raises at the genexp site.
try:
    bad = (x for x in 5)
except TypeError as e:
    print("outer:", e)

# An inner iterable failing raises only at the first next.
gi = (y for x in [10, 20] for y in x)
print("inner built")
try:
    next(gi)
except TypeError as e:
    print("inner:", e)

# A genexp inside a def carries a qualified name but behaves the same.
def make():
    base = 10
    return (base + k for k in range(3))

print(list(make()))
