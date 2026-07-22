# sys.builtin_module_names lists the statically linked modules as a sorted tuple
# of strings; posix and sys are always present on this platform. The filesystem
# codec is UTF-8 with surrogateescape, the pair os.py's fsencode builds on.
import sys

n = sys.builtin_module_names
print(type(n).__name__, "posix" in n, "sys" in n)
print(all(isinstance(x, str) for x in n), list(n) == sorted(n))
print(sys.getfilesystemencoding(), sys.getfilesystemencodeerrors())
