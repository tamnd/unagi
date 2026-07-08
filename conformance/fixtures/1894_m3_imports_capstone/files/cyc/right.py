print("right start")
from . import left

try:
    print("left.lval mid-cycle:", left.lval)
except AttributeError as e:
    print("attr:", e)

rval = "R"
print("right done")
