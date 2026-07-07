# os.path is the path-manipulation half of os, provided in Go behind the os.path
# import at floor scope. The string routines are pure and deterministic, so they
# go straight in the golden; the stat probes run against paths the fixture makes
# so exists/isfile/isdir stay stable.
import os
import os.path as osp

print("join:", os.path.join("a", "b", "c"))
print("join abs:", os.path.join("a", "/b", "c"))
print("join trail:", os.path.join("a/", "b"))
print("join empty tail:", repr(os.path.join("a", "")))

print("split:", os.path.split("a/b/c"))
print("split root:", os.path.split("/"))
print("split trail:", os.path.split("a/b/"))

print("splitext:", os.path.splitext("a/b.txt"))
print("splitext none:", os.path.splitext("a/b"))
print("splitext dotfile:", os.path.splitext(".bashrc"))
print("splitext double:", os.path.splitext("a.tar.gz"))

print("basename:", os.path.basename("a/b/c"), repr(os.path.basename("a/b/")))
print("dirname:", os.path.dirname("a/b/c"), repr(os.path.dirname("a")))

print("isabs:", os.path.isabs("/a"), os.path.isabs("a"))

print("normpath:", os.path.normpath("a/./b/../c"))
print("normpath slashes:", os.path.normpath("a//b"))
print("normpath up:", os.path.normpath("../a"))
print("normpath dbl:", repr(os.path.normpath("//a/b")))
print("normpath triple:", repr(os.path.normpath("///a/b")))
print("normpath empty:", repr(os.path.normpath("")))

print("commonprefix:", repr(os.path.commonprefix(["/usr/lib", "/usr/local"])))
print("commonprefix empty:", repr(os.path.commonprefix([])))

print("as-alias join:", osp.join("x", "y"))
print("sep:", os.path.sep)

# The stat probes over paths every POSIX host has: the root directory, a
# character device, and a name that cannot exist.
print("exists root:", os.path.exists("/"))
print("isdir root:", os.path.isdir("/"))
print("isfile root:", os.path.isfile("/"))
print("exists devnull:", os.path.exists("/dev/null"))
print("isfile devnull:", os.path.isfile("/dev/null"))
print("exists missing:", os.path.exists("/no/such/unagi/path"))
print("isdir missing:", os.path.isdir("/no/such/unagi/path"))
