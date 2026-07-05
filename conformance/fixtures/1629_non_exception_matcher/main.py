# A non-exception class used as an except matcher raises TypeError, even when a
# valid matcher in the same clause would have caught the exception, matching
# CPython which validates the whole handler before it matches. The TypeError is
# chained on the exception being handled.
class Plain:
    pass


class AppError(Exception):
    pass


def single():
    try:
        try:
            raise KeyError("k")
        except Plain:
            print("unexpected")
    except TypeError as te:
        print("single", te, "context:", type(te.__context__).__name__)


single()


def tuple_with_bad():
    try:
        try:
            raise ValueError("v")
        except (ValueError, Plain):
            print("unexpected")
    except TypeError as te:
        print("tuple", te, "context:", type(te.__context__).__name__)


tuple_with_bad()

# Every matcher a real exception class still matches normally, so a valid user
# subclass alongside a built-in in one tuple is caught.
try:
    raise AppError("ok")
except (KeyError, AppError) as e:
    print("valid tuple", type(e).__name__, e)
