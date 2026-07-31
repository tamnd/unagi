import errno

# OSError's constructor splits 2..5 positional arguments into the named members
# errno, strerror, filename and filename2, collapses args to the (errno, strerror)
# pair, and — for a bare OSError with an integer errno — remaps to the matching
# subclass. str() then renders the "[Errno N] strerror: 'filename' -> 'filename2'"
# form. All of this is host-independent: it is Python-level object construction,
# with no syscall, so the errno numbers below are the portable ones from the errno
# module rather than raw integers.


def show(e):
    print(
        type(e).__name__,
        "|",
        str(e),
        "|",
        e.errno,
        e.strerror,
        e.filename,
        e.filename2,
        "|",
        e.args,
    )


# Two args: errno + strerror, no filename. Bare OSError(ENOENT, ...) remaps.
show(OSError(errno.ENOENT, "No such file or directory"))
# Three args: adds filename.
show(OSError(errno.ENOENT, "No such file or directory", "x.txt"))
# Five args: the winerror slot (arg 4) is dropped on this path, filename2 kept.
show(OSError(errno.ENOENT, "msg", "src", 0, "dst"))
# EACCES remaps to PermissionError.
show(OSError(errno.EACCES, "Permission denied", "y"))
# An unmapped integer errno stays a plain OSError.
show(OSError(9999, "weird error"))
# A non-integer errno stays OSError and still splits into the members.
show(OSError("notanint", "msg"))

# Argument counts CPython leaves whole: zero, one, and more than five keep the
# generic shape — the members read None and str is the empty/one-arg/tuple form.
show(OSError())
show(OSError("boom"))
show(OSError(1, 2, 3, 4, 5, 6))

# A directly constructed subclass keeps its own class but still splits its args.
show(FileNotFoundError(errno.ENOENT, "nope", "z"))

# The remap picks the right subclass across the common errno values, and the
# result is catchable as that subclass and as OSError.
for name in ("ENOENT", "EACCES", "EEXIST", "ENOTDIR", "EISDIR", "EPERM"):
    code = getattr(errno, name)
    try:
        raise OSError(code, "boom", "f")
    except OSError as e:
        print(name, "->", type(e).__name__, "|", str(e), "| errno", e.errno == code)

# repr collapses to the (errno, strerror) pair, dropping the filename.
print(repr(OSError(errno.ENOENT, "No such file or directory", "gone.txt")))
