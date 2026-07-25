# On 3.14 a nan hashes by object identity, not to the pre-3.10 constant 0, so
# two distinct nan objects occupy separate dict slots and set elements while the
# same nan object still finds its own slot.

n1 = float('nan')
n2 = float('nan')

d = {n1: "a", n2: "b"}
print(len(d))
print(d[n1], d[n2])
print(n1 in d)
print(float('nan') in d)

s = {n1, n2}
print(len(s))
print(n1 in s)

# The identity hash is not the old 0, and two distinct nans differ.
print(hash(n1) == 0)
print(hash(n1) == hash(n2))
print(hash(n1) == hash(n1))

# Ordinary float hashing is untouched: equal to the int, integral collisions.
print(hash(1.0) == hash(1))
print(hash(2.5) == hash(2.5))
print(len({0.0, -0.0}))
