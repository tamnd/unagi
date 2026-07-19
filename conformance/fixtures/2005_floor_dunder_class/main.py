# __class__ on the scalar and container builtins reports type(x), the same type
# object the type() builtin yields. _py_abc's __instancecheck__ opens with
# `subclass = instance.__class__`, so every value has to answer this for the ABC
# machinery collections.abc leans on to run. unagi keeps no per-value class
# pointer for these, so the read routes through the same TypeOf the type() call
# uses.
values = [42, "s", [1], (1,), {"k": 1}, {1, 2}, 1.5, True, None, b"by", 3 + 4j]
for v in values:
    cls = v.__class__
    print(cls, cls.__name__, cls is type(v))

# __class__ and type() name the very same object, and it round-trips through
# isinstance, so the ABC __instancecheck__ read is well founded.
print((42).__class__ is int)
print(isinstance("s", "s".__class__))
print([].__class__ is list)
print(True.__class__ is bool)
print(None.__class__.__name__)
