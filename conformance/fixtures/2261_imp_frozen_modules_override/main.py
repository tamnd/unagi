# test.support.import_helper.frozen_modules() toggles _imp's frozen-module
# override around a body, and import_fresh_module() drives it to reimport a
# module from scratch. The override must exist and return None or entering the
# context manager raises AttributeError -- the gate a broad swath of Lib/test
# modules pass through. unagi freezes nothing, so the toggle is a no-op, but the
# surface has to be present and well behaved.
import _imp
from test.support import import_helper

# The primitive exists and each documented argument returns None.
print(hasattr(_imp, "_override_frozen_modules_for_tests"))
print(_imp._override_frozen_modules_for_tests(1))
print(_imp._override_frozen_modules_for_tests(-1))
print(_imp._override_frozen_modules_for_tests(0))

# The context manager enters and exits cleanly in both directions.
with import_helper.frozen_modules(True):
    print("enabled body")
with import_helper.frozen_modules(False):
    print("disabled body")

# import_fresh_module drives frozen_modules under the hood and hands back a live
# module.
math = import_helper.import_fresh_module("math")
print(math.factorial(5), math.gcd(12, 18))
