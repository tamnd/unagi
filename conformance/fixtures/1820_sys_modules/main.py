# sys.modules is the live import registry, not a copy: a poke satisfies the
# next import, a None entry halts it, a delete makes the body run again, and
# a module that deletes its own entry surfaces the registry's KeyError.
import sys

print("name:", sys.__name__)
print("has file:", hasattr(sys, "__file__"))
print("package:", repr(sys.__package__))
print("modules is dict:", type(sys.modules).__name__)
print("sys registered:", "sys" in sys.modules)

# first import runs the body, the second is a registry hit
import counter
print("n:", counter.n)
import counter
print("still:", counter.n)

# deleting the entry forces a re-exec
del sys.modules["counter"]
import counter
print("again:", counter.n)

# a None entry halts the import
sys.modules["halted"] = None
try:
    import halted
except ImportError as e:
    print("halted:", e)

# a poked object satisfies the import without any file behind it
class Fake:
    marker = 99

sys.modules["never_on_disk"] = Fake()
import never_on_disk
print("poked:", never_on_disk.marker)
from never_on_disk import marker
print("from poked:", marker)

# a body that deletes its own entry fails the import with the raw KeyError
try:
    import selfdel
except KeyError as e:
    print("selfdel:", type(e).__name__, e)
