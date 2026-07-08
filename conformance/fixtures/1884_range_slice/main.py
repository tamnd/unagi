# Slicing a range yields another range, lazily, the way CPython does, so
# range(len(x))[::-1] walks the indices backward without building a list.
# re._parser leans on exactly that when it rewrites a subpattern in reverse.

r = range(10)
print(r[::-1])
print(r[2:8])
print(r[2:8:2])
print(list(r[::-1]))
print(list(r[2:8:2]))

# The step composes and the bounds clamp, both the CPython way.
print(range(0, 20, 3)[1:5], list(range(0, 20, 3)[1:5]))
print(range(5)[::-2], list(range(5)[::-2]))
print(list(range(10)[-3:]))
print(list(range(10)[100:200]))
print(list(range(10)[-100:100]))

# An empty result is a zero-length range, and a slice object reads the same.
print(range(10)[8:2])
print(range(10)[slice(2, 8, 2)])

# A negative step over a negative-step range keeps the arithmetic honest.
print(list(range(20, 0, -2)[::-1]))
print(len(range(100)[10:90:3]))

# The step-zero and non-integer errors match the other sequences.
try:
    range(10)[::0]
except ValueError as e:
    print(e)
try:
    range(10)['a':]
except TypeError as e:
    print(e)
