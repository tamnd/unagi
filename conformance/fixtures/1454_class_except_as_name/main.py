# An except handler's as-name in a class body binds through the class namespace
# and is unbound when the handler exits, the way CPython compiles it as
# `name = None; del name` in a finally. A later read of the name in the body
# falls through and raises NameError, and the class object never carries it.
# The unbind runs even when the handler body raises on its way out.


class Caught:
    try:
        raise ValueError("boom")
    except ValueError as e:
        captured = str(e)

    # The as-name is gone once the handler exits.
    try:
        leaked = e
        after = "leaked"
    except NameError:
        after = "unbound"


class Reraised:
    outcome = "none"
    try:
        try:
            raise KeyError("k")
        except KeyError as err:
            raise RuntimeError("in handler")
    except RuntimeError:
        outcome = "reraised"

    # The unbind still ran, so err is not bound.
    try:
        z = err
        r = "leaked"
    except NameError:
        r = "unbound"


print(Caught.captured)
print(Caught.after)
print(hasattr(Caught, "e"))
print(Reraised.outcome)
print(Reraised.r)
print(hasattr(Reraised, "err"))
