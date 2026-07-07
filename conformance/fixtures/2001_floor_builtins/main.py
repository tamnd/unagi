# builtins is the module behind the names a program reaches without importing
# anything: the functions, the type objects, the exception classes, and the
# constants. Floor modules import it to reach a builtin through a stable name
# even where a local one shadows it, so this drives that surface directly.
import builtins

# The plain builtin functions, reached through the module.
print(builtins.len("abcd"), builtins.abs(-7), builtins.max(3, 9, 1), builtins.min([4, 2, 8]))
print(builtins.bin(10), builtins.oct(10), builtins.hex(255))
print(builtins.sorted([3, 1, 2]), builtins.list(builtins.range(3)))
print(builtins.str(12), builtins.int("42"), builtins.abs(-1.5))
print(builtins.isinstance(1, int), builtins.issubclass(bool, int))

# The exception classes are the same objects the bare names bind.
print(builtins.ValueError is ValueError, builtins.KeyError.__name__)

# None, True, and False are keywords, so they are reachable only through
# getattr, but they are attributes of the module the way CPython carries them.
print(getattr(builtins, "None") is None, getattr(builtins, "True"), getattr(builtins, "False"))
print(builtins.__debug__, repr(builtins.NotImplemented), builtins.Ellipsis is ...)

# The descriptor constructors work as decorators through the module too.
class C:
    @builtins.staticmethod
    def s():
        return "static"

    @builtins.classmethod
    def c(cls):
        return cls.__name__

    @builtins.property
    def p(self):
        return 99


print(C.s(), C.c(), C().p)

# A builtin called wrong raises the same error the bare name would.
try:
    builtins.len(5)
except TypeError:
    print("typeerror caught")
