import unittest


class MyErr(Exception):
    pass


class SubErr(MyErr):
    pass


# The first argument a with statement hands __exit__ on the exception path is the
# raised exception's real class, so identity and issubclass against the user
# class hold, the same object type(e) and except matching key on.
seen = {}


class Probe:
    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_value, tb):
        seen["is"] = exc_type is MyErr
        seen["sub_is_myerr"] = issubclass(exc_type, MyErr)
        seen["is_exception"] = issubclass(exc_type, Exception)
        seen["name"] = exc_type.__name__
        return True


with Probe():
    raise MyErr("boom")
print("user exc_type is class:", seen["is"])
print("user issubclass Exception:", seen["is_exception"])
print("user exc_type name:", seen["name"])

with Probe():
    raise SubErr("boom")
print("subclass issubclass base:", seen["sub_is_myerr"])
print("subclass exc_type name:", seen["name"])


# A built-in exception still resolves to its built-in class.
class BuiltinProbe:
    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_value, tb):
        print("builtin exc_type is ValueError:", exc_type is ValueError)
        print("builtin issubclass Exception:", issubclass(exc_type, Exception))
        return True


with BuiltinProbe():
    raise ValueError("v")


# unittest.assertRaises checks issubclass(exc_type, expected) under the hood, so
# it now matches a user-defined exception raised by the callable.
def raises_user():
    raise MyErr("x")


def raises_sub():
    raise SubErr("y")


tc = unittest.TestCase()
tc.assertRaises(MyErr, raises_user)
print("assertRaises user match: ok")
tc.assertRaises(MyErr, raises_sub)
print("assertRaises subclass match: ok")
with tc.assertRaises(MyErr):
    raise MyErr("z")
print("assertRaises context match: ok")

# sys.exc_info inside an except block reports the same real class.
try:
    raise SubErr("w")
except MyErr:
    import sys

    et = sys.exc_info()[0]
    print("exc_info type is SubErr:", et is SubErr)
    print("exc_info issubclass MyErr:", issubclass(et, MyErr))
