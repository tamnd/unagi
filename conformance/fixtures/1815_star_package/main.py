# A star import over a package with no __all__ takes the default rule against
# the package namespace: the name the __init__ bound and the submodule it
# pulled in both transfer, while an underscore name stays behind.
from pkg import *

print("name:", name)
print("sub:", sub.value)
try:
    print(_secret)
except NameError as e:
    print("secret:", type(e).__name__, e)
