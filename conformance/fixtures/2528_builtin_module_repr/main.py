import sys
import builtins


def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as e:
        print(label, type(e).__name__, ":", e)


# A built-in module reprs from its spec origin, not a filesystem path.
show("sys", lambda: sys)
show("builtins", lambda: builtins)

# The same modules reached through a builtin function's __self__ repr the same
# way, since __self__ is the owning module.
show("len.__self__", lambda: len.__self__)
show("print.__self__", lambda: print.__self__)

# The __name__ still reads back plainly.
print("sys name", sys.__name__)
print("builtins name", builtins.__name__)

# builtin_module_names includes the genuine built-ins.
print("sys builtin", "sys" in sys.builtin_module_names)
print("builtins builtin", "builtins" in sys.builtin_module_names)
