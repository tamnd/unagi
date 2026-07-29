# sys._jit is PEP 744's introspection surface for the experimental JIT. A
# compiled program is ahead-of-time with no JIT tier, so all three predicates
# report False. test.support reads _JIT_ENABLED = sys._jit.is_enabled() at
# import, so this is one of the small gaps on the path to importing it.
import sys

print("available", sys._jit.is_available())
print("enabled", sys._jit.is_enabled())
print("active", sys._jit.is_active())

# The exact form test.support keys on.
_JIT_ENABLED = sys._jit.is_enabled()
print("_JIT_ENABLED", _JIT_ENABLED)

# Each predicate is a no-argument callable resolvable off the namespace and by
# getattr, and returns a genuine bool.
f = sys._jit.is_enabled
print("bound", f())
print("getattr", getattr(sys._jit, "is_available")())
print("types", type(sys._jit.is_active()).__name__,
      type(sys._jit.is_enabled()).__name__,
      type(sys._jit.is_available()).__name__)
