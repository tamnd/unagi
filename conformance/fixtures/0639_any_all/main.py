# any/all over a range of iterables and their truthiness.
print(any([0, 0, 1]), any([]), any([0, ""]))
print(all([1, 2, 3]), all([]), all([1, 0, 2]))

# Strings, sets and dicts iterate too; a dict tests its keys.
print(any("abc"), all(""), any({0, 0}), all({1, 2}))
print(any({0: "z"}), all({0: "z", 1: "y"}))

# Both short-circuit: the generator is only advanced up to the decision.
def gen(stop, hit):
    for i in range(5):
        print("step", i)
        yield hit if i == stop else (0 if hit else 1)

print(any(gen(2, 1)))
print(all(gen(1, 0)))

# A non-iterable argument is a TypeError, catchable as usual.
try:
    any(5)
except TypeError as e:
    print("any error:", e)
try:
    all(3.0)
except TypeError as e:
    print("all error:", e)

# Passed around as values.
a, b = any, all
print(a([0, 1]), b([1, 1]))
