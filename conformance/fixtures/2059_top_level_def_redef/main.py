# A top-level function redefinition where the first binding is not conditional,
# the shape locale uses when it aliases localeconv and then wraps it with a
# decorated redefinition. A redefined name resolves through its module variable,
# so the def statements assign it in source order and a call reads the last one
# to take effect.

import functools


# The locale idiom: a guarded fallback def in a branch that does not run, the
# live binding aliased, then a top-level decorated redefinition that wraps the
# alias. The wrapper is the final binding and always takes effect.
USE_FALLBACK = False

if USE_FALLBACK:

    def conv():
        return {"src": "fallback"}

else:

    def conv():
        return {"src": "native"}


_conv = conv


@functools.wraps(_conv)
def conv():
    d = _conv()
    d["wrapped"] = True
    return d


print(conv())
print(conv.__name__)


# A plain double definition at the top level, both unconditional. The second
# wins; every read goes through the variable.
def greet(name):
    return "hi " + name


def greet(name):
    return "hello " + name


print(greet("sam"))


# A call between the two definitions observes the first, a call after observes
# the second, since the name resolves dynamically in source order.
def pick():
    return "one"


print(pick())


def pick():
    return "two"


print(pick())
