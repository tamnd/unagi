# A lambda whose own scope contains a yield is a generator function: calling it
# builds a generator, and the body expression's value becomes the
# StopIteration value. This is the shape _collections_abc uses to name the
# generator type.
gen = (lambda: (yield))()
print(type(gen).__name__)
print(next(gen))
try:
    gen.send(99)
except StopIteration as e:
    print("stop", e.value)

# Parameters bind as usual, and the yielded value flows through.
h = (lambda x: (yield x))(7)
print(next(h))
try:
    h.send("sent")
except StopIteration as e:
    print("stop2", repr(e.value))

# yield in a larger body expression; drive it to exhaustion.
def make(seq):
    return lambda: (yield from seq)

g = make([1, 2, 3])()
print(list(g))

# A plain lambda stays an ordinary function.
f = lambda a: a + 1
print(f(4), type(f).__name__)

# The generator type is the same object CPython names.
import types
print(type(gen) is types.GeneratorType)
