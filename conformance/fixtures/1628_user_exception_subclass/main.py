# A user exception subclass raises, catches, and reports its identity the way
# CPython's built-in exceptions do, so raise/except work across the boundary
# between built-in bases and user-defined exception classes.
class AppError(Exception):
    pass


class NotFound(AppError):
    pass


class Other(Exception):
    pass


def handle(exc):
    try:
        raise exc
    except NotFound as e:
        print("notfound", type(e).__name__, e)
    except AppError as e:
        print("apperror", type(e).__name__, e)
    except (Other, ValueError) as e:
        print("tuple", type(e).__name__, e)
    except Exception as e:
        print("base", type(e).__name__, e)


handle(NotFound("missing"))
handle(AppError("bad"))
handle(Other("x"))
handle(ValueError("v"))
handle(KeyError("k"))

# Construction as a plain value, not raised: args, str, and repr follow the
# default BaseException behaviour.
e = AppError("built", 42)
print(e.args)
print(str(e))
print(repr(e))
print(isinstance(e, Exception), isinstance(e, AppError), isinstance(e, NotFound))
print(issubclass(NotFound, AppError), issubclass(NotFound, Exception))
print(type(e) is AppError, type(e).__name__)

# A bare class raises a no-argument instance.
try:
    raise NotFound
except AppError as e:
    print("bare", type(e).__name__, e.args)

# Implicit context chaining keeps the class identity of both exceptions.
try:
    try:
        raise AppError("first")
    except AppError:
        raise NotFound("second")
except Exception as e:
    print("chain", type(e).__name__, type(e.__context__).__name__)
