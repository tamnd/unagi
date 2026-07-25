# globals and dir used as values (aliased, not called in place) crashed the
# runtime: the compiler lowers a literal globals()/dir() call against the module,
# but a value read emitted BuiltinFn("globals"), which had no registered object.
# timeit does exactly this at import: `_globals = globals`. Both now resolve to a
# real callable that reads the top-level module's namespace, the honest best a
# frameless AOT runtime can do. Everything printed here is host-invariant: type
# names, membership booleans, and a value stored into and read back from globals.

MARKER = 41


def alias_globals():
    g = globals  # the timeit idiom: bind the builtin as a value
    return g


def alias_dir():
    return dir


# globals aliased and called returns the module dict; MARKER is visible in it.
g = alias_globals()
ns = g()
print("globals type", type(ns).__name__)
print("marker in globals", "MARKER" in ns)
print("marker value", ns["MARKER"])

# The returned namespace is a live mutable dict.
ns["INJECTED"] = 99
print("injected read back", ns["INJECTED"])

# An argument to the value form raises the CPython TypeError.
try:
    g(1)
except TypeError as e:
    print("globals arg", type(e).__name__)


class Widget:
    kind = "button"

    def press(self):
        return self.kind


# dir aliased and called on an object lists that object's names.
d = alias_dir()
wnames = d(Widget)
print("press in dir(Widget)", "press" in wnames)
print("kind in dir(Widget)", "kind" in wnames)
print("dir(Widget) sorted", wnames == sorted(wnames))

# dir aliased with no argument lists the top-level module's names.
mnames = d()
print("MARKER in dir()", "MARKER" in mnames)
print("Widget in dir()", "Widget" in mnames)

# import timeit no longer crashes on `_globals = globals` at module scope.
import timeit

print("timeit imported", timeit.__name__)
print("default_repeat", timeit.default_repeat)

print("done")
