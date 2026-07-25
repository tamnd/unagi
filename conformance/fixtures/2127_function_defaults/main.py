# A function exposes __defaults__ and __kwdefaults__, the trailing positional
# defaults as a tuple and the keyword-only defaults as a dict, so inspect can
# rebuild a signature. Both read None when the function has none of that kind.
import inspect


def add(a, b, c=0, *args, d=1, e=2, **kw):
    return a + b + c


def plain(x, y):
    return x


def posonly(x, /, y, z=3):
    return x


# The trailing positional defaults come back as a tuple in declaration order.
print(add.__defaults__)
print(add.__kwdefaults__)

# No defaults of a kind reads None, not an empty container.
print(plain.__defaults__, plain.__kwdefaults__)

# A positional-only parameter counts toward __defaults__ the same way.
print(posonly.__defaults__)

# inspect.signature pairs the defaults back onto the last parameters and reads
# the keyword-only tail from __kwdefaults__.
sig = inspect.signature(add)
print(str(sig))
print([p.name for p in sig.parameters.values()])
print(sig.parameters["c"].default, sig.parameters["d"].default)

# A missing __text_signature__ is the None getattr default inspect relies on.
print(getattr(add, "__text_signature__", None))
