# The native _warnings module: importlib._bootstrap._setup loads it by name
# through the BuiltinImporter, so it has to be in sys.builtin_module_names and
# resolve to a module. It exposes warn and warn_explicit, which delegate to the
# public warnings module so a direct call filters and records the same way.

import sys

# _warnings is a builtin module, the way _thread and _weakref already are.
print("_warnings" in sys.builtin_module_names)
print("_thread" in sys.builtin_module_names, "_weakref" in sys.builtin_module_names)

import _warnings

print(type(_warnings).__name__)
print(callable(_warnings.warn), callable(_warnings.warn_explicit))

import warnings

# The public warnings module keeps its own behavior: a recorded warning carries
# the message and category the caller passed.
with warnings.catch_warnings(record=True) as caught:
    warnings.simplefilter("always")
    warnings.warn("public path", DeprecationWarning)
    print(len(caught), caught[0].category.__name__, str(caught[0].message))

# _warnings.warn reaches the same recording machinery.
with warnings.catch_warnings(record=True) as caught:
    warnings.simplefilter("always")
    _warnings.warn("native path", UserWarning)
    print(len(caught), caught[0].category.__name__, str(caught[0].message))
