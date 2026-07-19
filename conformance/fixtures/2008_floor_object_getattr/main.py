# Attribute access on a bare object() runs object's own default protocol, not a
# user hook. A plain object() instance has the object root as its class, which
# carries the __getattribute__/__setattr__/__delattr__ slot wrappers, so the
# trampoline has to tell object's default from a real override or it calls the
# default wrapper as if a user had defined it. That misfire raised
# `__getattribute__() takes 2 positional arguments but 1 was given` on the very
# first getattr, which is what ABCMeta.__new__ runs over a class namespace with
# `getattr(value, "__isabstractmethod__", False)`, so _collections_abc could not
# import.
m = object()

# getattr with a default falls back cleanly on a miss.
print(getattr(m, "__isabstractmethod__", False))
print(getattr(m, "missing", "dflt"))

# __class__ and hasattr read through the generic core.
print(hasattr(m, "__class__"), m.__class__ is object)

# The ABCMeta.__new__ abstract-method comprehension over a namespace of mixed
# values runs the same getattr the misfire broke.
ns = {"m": m, "n": 5, "s": "x"}
print({k for k, v in ns.items() if getattr(v, "__isabstractmethod__", False)})

# A write goes to object's default __setattr__, which rejects it since a bare
# object carries no __dict__, and a genuine miss raises the standard message.
try:
    m.x = 1
except AttributeError as e:
    print("setattr:", "no attribute" in str(e))
try:
    getattr(m, "nope")
except AttributeError as e:
    print("getattr raise:", "has no attribute 'nope'" in str(e))
