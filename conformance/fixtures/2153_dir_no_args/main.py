# dir() with no arguments at module scope lists the module namespace, the idiom
# stdlib modules use to build __all__.
import sys

ALPHA = 1
BETA = 2
_private = 99

def gamma():
    return 1

class Delta:
    def __init__(self):
        self.x = 1

# __all__-style construction: uppercase public names.
built = [x for x in dir() if x.isupper() and not x.startswith('_')]
print("public upper:", built)

# Membership of every kind of module-level name.
print("ALPHA in dir():", "ALPHA" in dir())
print("gamma in dir():", "gamma" in dir())
print("Delta in dir():", "Delta" in dir())
print("sys in dir():", "sys" in dir())
print("_private in dir():", "_private" in dir())

# dir() returns a sorted list.
names = dir()
print("sorted:", names == sorted(names))
print("is list:", isinstance(names, list))

# dir(x) with an argument still lists the object's attributes.
d = Delta()
print("x in dir(d):", "x" in dir(d))

# A name bound after the earlier dir() calls shows up in a later dir().
EPSILON = 3
print("EPSILON in dir():", "EPSILON" in dir())
print("done")
