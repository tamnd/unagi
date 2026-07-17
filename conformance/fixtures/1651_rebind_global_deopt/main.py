# boost reads the module global FACTOR, so its static form reads a typed shadow
# behind a binding-version guard hoisted to the function entry. While FACTOR
# stays the int it was bound to, the guard passes and the static form multiplies
# through the shadow. wreck rebinds FACTOR to a float the int shadow cannot hold,
# which bumps the binding version, so the next call fails the entry guard and
# deopts to the boxed body. The boxed body reads the live float binding, so the
# product is a float. CPython has no tiers and simply reads whatever FACTOR holds
# at each call, so it prints the same int then float; both unagi tiers must land
# on those exact bytes.
FACTOR = 3


def boost(x: int) -> int:
    return x * FACTOR


def wreck():
    global FACTOR
    FACTOR = 3.5


print(boost(5))
print(boost(10))
wreck()
print(boost(4))
print(boost(2))
