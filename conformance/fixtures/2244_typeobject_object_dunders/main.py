# A constructor-less builtin type object (code, generator, function, ...) sits
# at the head of a (T, object) MRO and inherits object's dunders as readable
# attributes, the same as a constructor-backed builtin type or a user class.
fn = lambda: 0
CodeType = type(fn.__code__)
GenType = type(x for x in [])
FuncType = type(fn)
MethodType = type("".upper)

INHERITED = [
    "__init__", "__eq__", "__ne__", "__str__", "__repr__", "__reduce__",
    "__reduce_ex__", "__subclasshook__", "__dir__", "__getstate__",
    "__lt__", "__le__", "__gt__", "__ge__", "__hash__",
]

for T, nm in [(CodeType, "code"), (GenType, "generator"),
              (FuncType, "function"), (MethodType, "builtin_method")]:
    flags = "".join(
        "1" if (callable(getattr(T, d)) or getattr(T, d) is None) else "0"
        for d in INHERITED
    )
    print(nm, flags)

# The object-inherited slots behave as object's: __init__ is a no-op and
# __subclasshook__ declines.
print("code_init", CodeType.__init__(object()) is None)
print("gen_hook", GenType.__subclasshook__(GenType) is NotImplemented)
