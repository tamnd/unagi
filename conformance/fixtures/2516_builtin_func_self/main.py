import builtins


def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# A plain builtin function reports the builtins module through __self__, the way
# CPython binds a builtin_function_or_method to the module it lives in, and the
# module read back is the very object `import builtins` binds. __name__ and
# __qualname__ stay the bare function name.
print("== a builtin function binds the builtins module ==")
for nm in [
    "len", "print", "abs", "sorted", "iter", "min", "max", "repr", "id", "hash",
    "chr", "ord", "isinstance", "issubclass", "getattr", "setattr", "hasattr",
    "callable", "next", "vars", "divmod", "pow", "round",
]:
    f = getattr(builtins, nm)
    show(f"{nm}.__self__ is builtins", lambda f=f: f.__self__ is builtins)

show("type(len.__self__).__name__", lambda: type(len.__self__).__name__)
show("len.__self__.__name__", lambda: len.__self__.__name__)
show("len.__name__", lambda: len.__name__)
show("len.__qualname__", lambda: len.__qualname__)

print("== a type constructor still has no __self__ ==")
show("int.__self__", lambda: int.__self__)
show("str.__self__", lambda: str.__self__)
show("list.__self__", lambda: list.__self__)

print("== the read works before an explicit import too ==")
show("chr.__self__.__name__", lambda: chr.__self__.__name__)
show("hash.__name__", lambda: hash.__name__)
