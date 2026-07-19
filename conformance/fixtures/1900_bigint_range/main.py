# range() with bounds that overflow int64 keeps the range whole instead of
# raising, so _collections_abc can take type(iter(range(1 << 1000))). The
# range never materializes: it constructs, iterates, reprs, tests membership,
# compares, indexes and hashes with big arithmetic, and len() of a range too
# large to count reports the honest overflow.
r = range(1 << 1000)
print(type(r).__name__)

it = iter(r)
print(next(it), next(it), next(it))

print(range(0, 1 << 70))
print(range(0, 1 << 70, 2))
print(range(1 << 70, 0, -3))

print((1 << 69) in range(0, 1 << 70))
print((1 << 70) in range(0, 1 << 70))
print((1 << 69) in range(0, 1 << 70, 2))
print((1 << 69) + 1 in range(0, 1 << 70, 2))

print(range(0, 1 << 70)[3])
print(range(0, 1 << 70)[-1])

print(range(1 << 70) == range(0, 1 << 70))
print(range(1 << 70) == range(1 << 70, 1 << 71))
print(hash(range(0)) == hash(range(1 << 70, 1 << 70)))

try:
    len(range(1 << 1000))
except OverflowError as e:
    print("len:", type(e).__name__)

print(next(iter(range(1 << 70, 0, -1))))

first = []
for x in range(1 << 200):
    first.append(x)
    if len(first) == 4:
        break
print(first)

# A small range built through the big path stays a plain fast range.
print(range(3)[1], list(range(3)))
