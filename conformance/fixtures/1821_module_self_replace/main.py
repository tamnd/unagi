# sys.modules[__name__] = obj inside a body makes every importer see obj: the
# import statement binds what the registry holds after the body finished, a
# from-import reads names as plain attributes of it, and a submodule's parent
# binding gets the replacement too.
import selfrep

print("type:", type(selfrep).__name__)
print("tag:", selfrep.tag)
print("call:", selfrep.hello())

from selfrep import hello

print("from:", hello())

# a missing name on the replacement is an ImportError with the
# unknown-module wording, since the instance has no __name__
try:
    from selfrep import absent
except ImportError as e:
    print("miss:", e)

# a dotted submodule that self-replaces: the parent attribute and the
# registry both hold the replacement
import pkg.sub

print("sub type:", type(pkg.sub).__name__)
print("sub tag:", pkg.sub.tag)
import sys

print("registry type:", type(sys.modules["pkg.sub"]).__name__)
