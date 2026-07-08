# M3 imports capstone: a regular package with relative imports, a namespace
# package with no __init__.py, and a circular import whose two halves observe
# each other mid-cycle through the live sys.modules registry.

import regpkg
from regpkg import greet
from regpkg.util import shout

print(regpkg.name)
print(greet("world"))
print(shout("go"))

# A namespace package (no __init__.py) resolves its submodule.
from nspace import part
print(part.describe())
import nspace.part as np
print(np.value)

# The circular import: importing cyc runs left, which imports right, which
# imports the still-initializing left and reads what has bound so far.
import cyc
print(cyc.left.use_right())
print(cyc.right.rval, cyc.left.lval)
print(cyc.left.__name__, cyc.right.__name__)

# The unresolvable case raises ModuleNotFoundError, caught and reported.
try:
    import nope_missing
except ModuleNotFoundError as e:
    print("missing:", e)
