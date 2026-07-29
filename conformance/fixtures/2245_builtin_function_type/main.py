# A builtin function (a C function or a bound builtin method) is a distinct kind
# from a Python def/lambda: it is types.BuiltinFunctionType, not
# types.FunctionType, and inspect.isbuiltin / isfunction key on exactly that.
import types
import inspect

def pyfunc():
    pass

lam = lambda: 0

# Unbound method descriptors (str.upper) and slot wrappers (object.__init__)
# are their own descriptor kinds in CPython; unagi models them coarsely as
# builtin_function_or_method, a separate pre-existing gap, so they are left out
# here — this fixture pins the def/lambda vs C-function vs type-constructor split.
CALLABLES = [
    ("pyfunc", pyfunc),
    ("lambda", lam),
    ("len", len),
    ("bound_builtin", "".upper),
    ("list.append", [].append),
    ("int_type", int),
]

for label, c in CALLABLES:
    print(
        label,
        "F" if isinstance(c, types.FunctionType) else "-",
        "L" if isinstance(c, types.LambdaType) else "-",
        "B" if isinstance(c, types.BuiltinFunctionType) else "-",
        "M" if isinstance(c, types.BuiltinMethodType) else "-",
        "isfunc" if inspect.isfunction(c) else "-",
        "isbuiltin" if inspect.isbuiltin(c) else "-",
    )

# FunctionType and BuiltinFunctionType are disjoint for these callables: nothing
# is both, matching CPython's split between a Python and a C callable.
print("disjoint", all(
    not (isinstance(c, types.FunctionType) and isinstance(c, types.BuiltinFunctionType))
    for _, c in CALLABLES
))
