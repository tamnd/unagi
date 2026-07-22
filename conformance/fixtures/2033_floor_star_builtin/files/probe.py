# A module (compiled in package mode) star-imports the posix builtin. posix has
# no compile-time export manifest, so the names bind at runtime through
# StarImportDynamic and later reads resolve through LoadModuleName.
from posix import *


def check():
    return (isinstance(getcwd(), str), getpid() > 0, callable(stat), callable(open))
