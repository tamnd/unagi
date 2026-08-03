# The traceback header qualifies a user exception's type with its __module__
# (errs.Outer.Inner) unless the module is builtins or __main__, where it prints
# the bare __qualname__. A class defined here in __main__ and a builtin both
# stay bare; an imported module's class, including a nested one, carries the
# module prefix in front of its qualified name.
import errs


class LocalError(Exception):
    pass


try:
    try:
        raise ValueError("builtin stays bare")
    except ValueError:
        raise LocalError("main stays bare")
except LocalError:
    raise errs.Outer.Inner("imported nested gets the prefix")
