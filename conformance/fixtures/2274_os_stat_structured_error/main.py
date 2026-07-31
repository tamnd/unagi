import os

# A failing stat-family call raises the structured OSError CPython does: remapped
# to the errno's subclass, carrying errno/strerror/filename, and rendering as
# "[Errno N] Strerror: 'filename'". The fixture runs in its own temp dir, so the
# relative names below are deterministic, and it sticks to ENOENT/ENOTDIR/EBADF
# whose strerror text is identical on the record and replay hosts.


def show(label, fn):
    try:
        fn()
    except OSError as e:
        print(label, "|", type(e).__name__, "|", str(e), "| errno", e.errno, "| fn", repr(e.filename))
    else:
        print(label, "| no-error")


# ENOENT -> FileNotFoundError, filename is the path as given, for str and bytes.
show("stat/missing", lambda: os.stat("nope"))
show("stat/missing-bytes", lambda: os.stat(b"nope"))
show("lstat/missing", lambda: os.lstat("nope"))
show("chmod/missing", lambda: os.chmod("nope", 0o644))

# ENOTDIR -> NotADirectoryError: a path that walks through a regular file.
with open("reg", "w") as f:
    f.write("x")
show("stat/notdir", lambda: os.stat("reg/child"))

# The fd forms: os.stat names the bad fd, os.fstat leaves the filename None.
show("stat/badfd", lambda: os.stat(99999))
show("fstat/badfd", lambda: os.fstat(99999))

# The remapped error is catchable as its specific subclass and carries the
# members off the caught object.
try:
    os.stat("nope")
except FileNotFoundError as e:
    print("catch FNF:", e.errno, e.strerror, repr(e.filename))

# A successful stat of the real file still works alongside the error paths.
st = os.stat("reg")
print("stat/ok size:", st.st_size)

os.remove("reg")
