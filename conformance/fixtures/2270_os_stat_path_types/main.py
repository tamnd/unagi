"""os.stat and its siblings accept every path spelling CPython does: a str, a
bytes path, an os.PathLike object reduced through __fspath__, and -- for stat
alone -- an integer file descriptor, which it fstats. lstat and access have no
descriptor form, so a non-path argument there is a TypeError, and a float is a
TypeError everywhere with the call's own message. Every printed line is a size,
a bool, or a fixed error string, so it is identical on every platform."""

import os


class FakePath:
    def __init__(self, path):
        self._path = path

    def __fspath__(self):
        return self._path


path = "probe.txt"
with open(path, "w", encoding="utf-8") as f:
    f.write("hello")

# str, bytes and os.PathLike all stat the same five-byte file.
print("stat str:", os.stat(path).st_size)
print("stat bytes:", os.stat(path.encode()).st_size)
print("stat pathlike:", os.stat(FakePath(path)).st_size)

# os.path.exists and os.access flow the same argument types through.
print("exists str/bytes/pathlike:",
      os.path.exists(path),
      os.path.exists(path.encode()),
      os.path.exists(FakePath(path)))
print("access:", os.access(path, os.F_OK), os.access(FakePath(path), os.R_OK))

# lstat over a path-like object matches.
print("lstat pathlike:", os.lstat(FakePath(path)).st_size)

# stat over an integer file descriptor fstats it; the mode is a plain int.
r, w = os.pipe()
try:
    print("stat(fd) mode is int:", isinstance(os.stat(r).st_mode, int))
finally:
    os.close(r)
    os.close(w)

# A float is rejected everywhere; lstat and access reject an fd too.
for label, call in [
    ("stat float", lambda: os.stat(1.5)),
    ("lstat float", lambda: os.lstat(1.5)),
    ("lstat int", lambda: os.lstat(0)),
    ("access float", lambda: os.access(1.5, 0)),
]:
    try:
        call()
        print(label, "-> no error")
    except TypeError as e:
        print(label, "->", e)

os.remove(path)
print("OK")
