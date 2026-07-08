print("left start")
from . import right
print("left resumed")
lval = "L"

def use_right():
    return right.rval
