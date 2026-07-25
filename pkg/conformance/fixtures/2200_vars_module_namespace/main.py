import types


# vars(module) is the module's live namespace dict, the same object __dict__
# hands back. sre_constants does globals().update(vars(re._constants)) at import,
# so a module argument has to answer instead of raising the __dict__ TypeError.
import re

d = vars(re)
print(type(d) is dict, "compile" in d, d is re.__dict__)


# sre_constants/sre_parse/sre_compile all import through that vars() hoist.
import sre_constants
import sre_parse
import sre_compile

print(sre_constants.MAGIC > 0, hasattr(sre_constants, "LITERAL"))


# vars(ns) is a SimpleNamespace's live attribute dict; a write through it reaches
# the namespace.
ns = types.SimpleNamespace(a=1, b=2)
v = vars(ns)
print(sorted(v.keys()), v["a"], v["b"], v is ns.__dict__)
v["c"] = 3
print(ns.c)


# vars() on something with no __dict__ still raises.
try:
    vars(42)
except TypeError as e:
    print("TypeError", "__dict__" in str(e))
