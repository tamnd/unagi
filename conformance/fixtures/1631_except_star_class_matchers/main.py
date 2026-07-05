# except* matches on class values the way plain except does: a user exception
# subclass is caught by itself or any base, a tuple flattens, and a matcher that
# is not an exception class raises the same TypeError, chained on the in-flight
# exception and validated even after the remainder is emptied.


class AppError(Exception):
    pass


class NotFound(AppError):
    pass


class Plain:
    pass


# A user subclass caught by its own class, bound as a one-element group.
try:
    raise NotFound("missing")
except* NotFound as eg:
    print("own", [type(e).__name__ for e in eg.exceptions], str(eg.exceptions[0]))


# A base catches every subclass leaf of a real group, leaving the rest behind.
try:
    raise ExceptionGroup("g", [NotFound("a"), ValueError("b"), AppError("c")])
except* AppError as eg:
    print("base", sorted(type(e).__name__ for e in eg.exceptions))
except* ValueError as eg:
    print("val", [type(e).__name__ for e in eg.exceptions])


# A tuple matcher flattens, so two unrelated classes both catch.
try:
    raise ExceptionGroup("g", [AppError("x"), ValueError("y")])
except* (AppError, ValueError) as eg:
    print("tuple", sorted(type(e).__name__ for e in eg.exceptions))


# A non-exception class matcher raises TypeError, chained on the exception it was
# handling, caught here so the program exits cleanly.
def bad_single():
    try:
        try:
            raise ValueError("v")
        except* Plain as eg:
            print("unexpected")
    except TypeError as te:
        print("bad", te, "context:", type(te.__context__).__name__)


bad_single()


# The bad matcher still raises when a valid earlier clause already caught
# everything and the remainder is empty.
def bad_after_empty():
    try:
        try:
            raise ValueError("v")
        except* ValueError as eg:
            print("first", [type(e).__name__ for e in eg.exceptions])
        except* Plain as eg:
            print("unexpected")
    except TypeError as te:
        print("empty-then-bad", te)


bad_after_empty()
