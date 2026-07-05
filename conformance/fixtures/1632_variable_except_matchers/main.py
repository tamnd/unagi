# An except matcher can be a variable holding an exception class or a tuple of
# them, evaluated at match time. A tuple flattens one level; a nested tuple
# element and a non-exception value both raise the same TypeError, chained on
# the exception being handled.


class AppError(Exception):
    pass


Base = AppError
pair = (KeyError, ValueError)
nested = (KeyError, (ValueError, AppError))


# A variable holding a class catches by that class.
try:
    raise AppError("boom")
except Base as e:
    print("var", type(e).__name__, e)


# A variable holding a tuple flattens to the union of its classes.
try:
    raise ValueError("v")
except pair as e:
    print("tuple-var", type(e).__name__)


# A nested tuple flattens one level only, so the inner tuple is not a class and
# raises TypeError, caught here.
def nested_bad():
    try:
        try:
            raise AppError("a")
        except nested as e:
            print("unexpected")
    except TypeError as te:
        print("nested-bad", te)


nested_bad()


# A non-exception variable raises the chained TypeError, caught here.
def bad():
    bad_matcher = 5
    try:
        try:
            raise ValueError("x")
        except bad_matcher as e:
            print("unexpected")
    except TypeError as te:
        print("bad-var", te, "context:", type(te.__context__).__name__)


bad()


# A variable matcher works after except* too, flattening the tuple.
try:
    raise ExceptionGroup("g", [ValueError("y"), KeyError("z")])
except* pair as eg:
    print("star-var", sorted(type(e).__name__ for e in eg.exceptions))
