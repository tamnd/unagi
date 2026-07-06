# Dotted imports over a regular package tree: ancestors run outward-in, each
# once, the plain form binds the root, `as` binds the leaf, and submodules
# hang off their parents as attributes afterward.
import pkg.sub.leaf

print(pkg.__name__, pkg.sub.__name__, pkg.sub.leaf.__name__)
print(repr(pkg.__package__), repr(pkg.sub.__package__), repr(pkg.sub.leaf.__package__))
print(pkg.sub.leaf.f())
print(pkg.val)

import pkg.sub.leaf as x

print(x is pkg.sub.leaf)

import pkg.sub

print("imports run once")
print(hasattr(pkg, "__path__"), hasattr(pkg.sub.leaf, "__path__"))


def g():
    import pkg.sub.leaf as inner

    return inner.f()


print(g())
