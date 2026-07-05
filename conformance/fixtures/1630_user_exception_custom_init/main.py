# User exception subclasses with custom __init__, super().__init__, attributes,
# and __str__/__repr__ overrides, caught and inspected.
class AppError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message

    def __str__(self):
        return f"[{self.code}] {self.message}"


class NotFound(AppError):
    def __init__(self, what):
        super().__init__(404, f"{what} not found")
        self.what = what


class Weird(Exception):
    tag = "W"

    def __repr__(self):
        return f"Weird<{self.args}>"

    def label(self):
        return f"{self.tag}:{self.args}"


def show(e):
    print("str", str(e))
    print("repr", repr(e))
    print("args", e.args)
    print("type", type(e).__name__)


try:
    raise NotFound("widget")
except AppError as e:
    show(e)
    print("code", e.code, "what", e.what)
    print("is AppError", isinstance(e, AppError))
    print("is Exception", isinstance(e, Exception))
    print("issubclass", issubclass(NotFound, AppError))
    print("dict", e.__dict__)

# __init__ that does not call super keeps constructor args
class Keep(Exception):
    def __init__(self, a, b):
        self.pair = (a, b)

k = Keep(1, 2)
print("Keep args", k.args, "pair", k.pair, "repr", repr(k))

# a raise inside __init__ propagates
class Bad(Exception):
    def __init__(self, x):
        if x < 0:
            raise ValueError("negative")
        super().__init__(x)

try:
    raise Bad(-1)
except ValueError as e:
    print("bad caught", e, "context", type(e.__context__).__name__)

# custom __repr__ only, plus a method and class var
w = Weird("a", "b")
print("weird repr", repr(w), "label", w.label())

# keyword args to a custom __init__
class KW(Exception):
    def __init__(self, *, code):
        super().__init__(code)
        self.code = code

kw = KW(code=7)
print("kw", kw.args, kw.code)

# post-hoc attribute on a builtin exception
v = ValueError("boom")
v.detail = "extra"
print("builtin", v.detail, vars(v))

# catching a subclass by the builtin base
try:
    raise NotFound("gadget")
except Exception as e:
    print("by base", type(e).__name__, str(e))
