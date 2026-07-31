import os

# os.chdir changes the process working directory and returns None. The fixture
# keeps every printed value platform-invariant: it works with relative names and
# basenames rather than the absolute cwd, which differs per host, and it sticks to
# ENOENT/ENOTDIR whose strerror text is identical on the record and replay hosts.

start = os.getcwd()

# Change into a freshly made subdirectory and read the change back through
# getcwd; the basename is the deterministic part of the absolute path.
os.mkdir("sub")
print("chdir ret:", repr(os.chdir("sub")))
print("basename:", os.path.basename(os.getcwd()))

# A relative "chdir up" returns to the starting directory.
os.chdir("..")
print("back:", os.getcwd() == start)

# The file-descriptor form fchdirs into an open directory.
fd = os.open("sub", os.O_RDONLY)
try:
    os.chdir(fd)
    print("fd basename:", os.path.basename(os.getcwd()))
finally:
    os.close(fd)
os.chdir(start)
os.rmdir("sub")


def show(label, fn):
    try:
        fn()
    except OSError as e:
        print(label, "|", type(e).__name__, "|", str(e), "| errno", e.errno, "| fn", repr(e.filename))
    else:
        print(label, "| no-error")


# A missing target -> FileNotFoundError naming the path, for str and bytes.
show("missing", lambda: os.chdir("nope"))
show("missing-bytes", lambda: os.chdir(b"nope"))

# A path that walks through a regular file -> NotADirectoryError.
with open("reg", "w") as f:
    f.write("x")
show("notdir", lambda: os.chdir("reg/child"))
os.remove("reg")

# An embedded NUL is screened before the syscall and raises ValueError.
try:
    os.chdir("a\x00b")
except ValueError as e:
    print("nul:", str(e))

# The remapped error is catchable as its specific subclass.
try:
    os.chdir("nope")
except FileNotFoundError as e:
    print("catch FNF:", e.errno, repr(e.filename))
