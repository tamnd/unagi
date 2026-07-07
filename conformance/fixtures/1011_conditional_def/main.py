# A module-level def inside a conditional block. The guarded-fallback idiom the
# standard library leans on: try the accelerator import, define a Python version
# when it is missing.

# The import fails, so the fallback def takes effect and its name calls through.
try:
    from _no_such_accelerator import helper
except ImportError:
    def helper(x):
        return x * 2

print(helper(21))

# A fallback with a default parameter: the default slot fills where the def runs.
try:
    from _no_such_accelerator import scale
except ImportError:
    def scale(x, factor=10):
        return x * factor

print(scale(5))
print(scale(5, factor=3))

# The import succeeds, so the fallback never binds and the imported name stays.
try:
    from math import floor as pick
except ImportError:
    def pick(x):
        return x

print(pick(3.7))

# Chosen at runtime through a plain if.
flag = True
if flag:
    def greet():
        return "hi"

print(greet())

# A branch that does not run leaves the name unbound, so a read raises NameError
# rather than resolving a function that was never defined on this path.
if False:
    def never():
        return 1

try:
    never()
except NameError:
    print("never is unbound")
