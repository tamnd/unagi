# copyreg compiled from the pinned CPython source into the floor. It is the
# first module re reaches at import that leans on both super and PEP 604 unions
# (it registers pickle(super, ...) and pickle(type(int | str), ...) at import),
# so getting it to import and run its registration surface is the last floor
# prerequisite before the re package itself.

import copyreg

print(copyreg.__name__)
print(sorted(copyreg.__all__))
print(type(copyreg.dispatch_table).__name__)

# complex is registered for reduction at import.
print(complex in copyreg.dispatch_table)
print(copyreg.pickle_complex(3 + 4j))


class Point:
    def __init__(self, x, y):
        self.x = x
        self.y = y


def pickle_point(p):
    return Point, (p.x, p.y)


# pickle() registers a reduction function for a type in the dispatch table.
copyreg.pickle(Point, pickle_point)
print(Point in copyreg.dispatch_table)
print(copyreg.dispatch_table[Point] is pickle_point)

# constructor() only validates that its argument is callable.
copyreg.constructor(Point)
try:
    copyreg.constructor(42)
except TypeError as e:
    print("constructor:", e)

# the extension registry round-trips a module, name, and code.
copyreg.add_extension("mymod", "MyClass", 240)
print(copyreg._extension_registry[("mymod", "MyClass")])
print(copyreg._inverted_registry[240])
copyreg.remove_extension("mymod", "MyClass", 240)
print(("mymod", "MyClass") in copyreg._extension_registry)

# __newobj__ allocates through cls.__new__ without running __init__.
p = copyreg.__newobj__(Point, 1, 2)
print(type(p).__name__)
print(hasattr(p, "x"))

# a code outside the 1..0x7fffffff range is rejected.
try:
    copyreg.add_extension("m", "N", 0)
except ValueError as e:
    print("code:", e)
