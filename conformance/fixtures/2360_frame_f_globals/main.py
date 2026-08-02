import sys

# A frame's f_globals is the defining module's namespace dict, the mapping
# sys._getframe walks read the same way CPython does. Referencing globals() here
# reifies the __main__ module so its frames carry that namespace (a plain script
# that never touches its globals keeps an empty f_globals, a documented
# divergence). f_globals carries __name__ and __file__, a nested function sees
# the same module name walking up the stack, and it is the very dict globals()
# returns. This is what warnings.warn and gettext key on to attribute a warning
# to the right module.
g = globals()

print("module name:", sys._getframe(0).f_globals.get("__name__"))
print("has __file__:", "__file__" in sys._getframe(0).f_globals)
print("is globals():", sys._getframe(0).f_globals is g)


def outer():
    def inner():
        fr = sys._getframe(0)
        print("inner name:", fr.f_globals.get("__name__"))
        print("caller name:", sys._getframe(1).f_globals.get("__name__"))
    inner()


outer()

# A write through f_globals carries into the module storage, so the caller frame
# a stdlib helper reaches can stash bookkeeping (warnings uses this for the
# per-module __warningregistry__).
sys._getframe(0).f_globals.setdefault("marker", 42)
print("marker:", g["marker"])
