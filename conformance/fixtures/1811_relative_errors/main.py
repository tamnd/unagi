# The two impossible relative forms raise when the statement executes: the
# entry script and a top-level module have no parent package, and extra dots
# past the tree top go beyond the top-level package. A function-body form
# only raises at call time.
try:
    from . import anything
except ImportError as e:
    print("entry:", type(e).__name__, e)

import toprel
import pkg.deep


def late():
    from . import never

    return never


print("late not called yet")
try:
    late()
except ImportError as e:
    print("late:", type(e).__name__, e)
