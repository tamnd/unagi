# posix's runtime export surface for os.py: cpu_count, getuid/geteuid, and the
# symlink/readlink round-trip posixpath.realpath drives. (posix also carries a
# synthesized __all__ so os.py's _get_exports_list has a list without a dir()
# builtin, but CPython's posix has none, so this fixture does not probe it.)
# Every assertion is a structural property identical on darwin and Linux; no
# uid, cpu count or errno number is ever printed.
import posix

print(posix.cpu_count() > 0)
print(isinstance(posix.getuid(), int), posix.getuid() >= 0, isinstance(posix.geteuid(), int))

posix.symlink("target.txt", "lnk")
print(posix.readlink("lnk"), posix.readlink(b"lnk") == b"target.txt")
try:
    posix.readlink("lnk_missing")
except OSError as e:
    print("missing:", type(e).__name__)
