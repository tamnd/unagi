# posix.scandir floor: the DirEntry surface and the iterator protocol.
# The cwd is scanned rather than a made dir (posix has no way to create a
# file until the fd slice), and only host-invariant properties are printed,
# so the golden is identical on the darwin oracle and the Linux corpus. The
# entry names themselves are never printed, only that they match listdir.
import posix

d = posix.getcwd()
listed = sorted(posix.listdir(d))

scanned = []
name_str = True
paths_ok = True
types_bool = True
inode_int = True
fspath_ok = True
junction_false = True

with posix.scandir(d) as it:
    for e in it:
        scanned.append(e.name)
        name_str = name_str and isinstance(e.name, str)
        # The path is the dir joined with the name; root keeps its single sep.
        paths_ok = paths_ok and (e.path == d + "/" + e.name or e.path == d + e.name)
        types_bool = types_bool and isinstance(e.is_dir(), bool)
        types_bool = types_bool and isinstance(e.is_file(), bool)
        types_bool = types_bool and isinstance(e.is_symlink(), bool)
        inode_int = inode_int and isinstance(e.inode(), int)
        fspath_ok = fspath_ok and e.__fspath__() == e.path
        junction_false = junction_false and e.is_junction() is False

print("names_match_listdir", sorted(scanned) == listed)
print("name_str", name_str)
print("paths_ok", paths_ok)
print("types_bool", types_bool)
print("inode_int", inode_int)
print("fspath_ok", fspath_ok)
print("junction_false", junction_false)
print("has_DirEntry", hasattr(posix, "DirEntry"))

# close makes the iterator stop even before it is exhausted.
it2 = posix.scandir(d)
it2.close()
try:
    next(it2)
    stopped = False
except StopIteration:
    stopped = True
print("stop_after_close", stopped)
