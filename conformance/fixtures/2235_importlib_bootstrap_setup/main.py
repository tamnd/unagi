import sys
import importlib
import importlib._bootstrap as boot

# The interpreter startup runs _bootstrap._setup(sys, _imp), which binds sys and
# _imp into the bootstrap namespace. Without them the pure import machinery
# raises NameError in _find_and_load the moment it reads sys.modules.
print(boot.sys is sys)
print(hasattr(boot, "_imp"))

# importlib.import_module drives _bootstrap._gcd_import and _find_and_load, the
# path a bare import statement does not take. A module not yet imported loads,
# and the object is the one the statement binds.
m = importlib.import_module("textwrap")
import textwrap
print(m is textwrap)
print(m.__name__)

# A dotted name resolves through the same machinery, driving the native
# meta_path finder for the submodule.
dec = importlib.import_module("collections.abc")
print(dec.__name__)

# import_module of an already-imported module returns the cached object.
print(importlib.import_module("textwrap") is textwrap)
