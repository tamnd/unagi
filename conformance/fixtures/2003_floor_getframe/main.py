# sys._getframe is the frame accessor _collections_abc reaches for at import to
# name the FrameLocalsProxy type: type(sys._getframe().f_locals) inside a
# function is that proxy. unagi compiles to Go and keeps no interpreter frames,
# so it maintains a lightweight shadow stack pushed and popped around every
# compiled call. This drives the frame surface the stdlib actually reads: the
# code object behind a frame, the caller chain through f_back, the optimized
# versus module f_locals split, and the too-deep ValueError. The live per-line
# f_lineno is a later slice and is not exercised here.
import sys


def inner():
    f = sys._getframe()
    print("co_name:", f.f_code.co_name)
    print("co_qualname:", f.f_code.co_qualname)
    print("co_firstlineno:", f.f_code.co_firstlineno)
    # A function frame's f_locals is a FrameLocalsProxy, the type the stdlib
    # registers as a Mapping.
    print("locals type:", type(f.f_locals).__name__)
    # f_back walks to the caller, so its code names the calling function.
    print("caller:", f.f_back.f_code.co_name)
    # sys._getframe(depth) reaches straight to an ancestor frame.
    print("depth1:", sys._getframe(1).f_code.co_name)
    print("depth2:", sys._getframe(2).f_code.co_name)


def outer():
    inner()


outer()

# The module frame is not optimized, so its f_locals is the namespace dict, not
# a proxy, exactly as CPython reports at module scope.
mod = sys._getframe()
print("module co_name:", mod.f_code.co_name)
print("module co_qualname:", mod.f_code.co_qualname)
print("module locals type:", type(mod.f_locals).__name__)
# Nothing calls the module body, so its f_back is None.
print("module f_back:", mod.f_back)

# A depth past the bottom of the stack is a ValueError, the guard CPython raises.
try:
    sys._getframe(1000)
except ValueError as e:
    print("ValueError:", e)


# The floor _collections_abc leans on: inside a function the f_locals type is
# FrameLocalsProxy, a type distinct from dict.
def framelocals_type():
    return type(sys._getframe().f_locals)


print("floor type:", framelocals_type().__name__)
print("floor is not dict:", framelocals_type() is not dict)
