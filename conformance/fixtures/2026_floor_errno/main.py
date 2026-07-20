# errno names the system error numbers os and io code raises and inspects. Each
# name is an integer attribute and errorcode maps a number back to its canonical
# name. The numbers are platform-specific, so this floor asserts only values that
# are identical across the oracle host and the corpus host (errno 1 through 34)
# plus structural properties (round-trips, the alias, the errorcode type). This
# is sub-slice 6a of the posix arc (Spec 2076 stdlib S0_posix_arc.md).
import errno

# the low errno numbers are the same on every POSIX host.
stable = [
    ("EPERM", 1), ("ENOENT", 2), ("ESRCH", 3), ("EINTR", 4), ("EIO", 5),
    ("ENXIO", 6), ("E2BIG", 7), ("ENOEXEC", 8), ("EBADF", 9), ("ECHILD", 10),
    ("ENOMEM", 12), ("EACCES", 13), ("EFAULT", 14), ("EBUSY", 16),
    ("EEXIST", 17), ("EXDEV", 18), ("ENODEV", 19), ("ENOTDIR", 20),
    ("EISDIR", 21), ("EINVAL", 22), ("ENFILE", 23), ("EMFILE", 24),
    ("ENOTTY", 25), ("EFBIG", 27), ("ENOSPC", 28), ("ESPIPE", 29),
    ("EROFS", 30), ("EMLINK", 31), ("EPIPE", 32), ("EDOM", 33), ("ERANGE", 34),
]
for name, num in stable:
    got = getattr(errno, name)
    print(name, got, got == num)

# errorcode is a dict from number to the canonical name, and it round-trips.
print("errorcode type:", type(errno.errorcode).__name__)
for name, num in stable:
    print(num, errno.errorcode[num], errno.errorcode[num] == name)

# EWOULDBLOCK aliases EAGAIN on every host, and the alias is kept out of
# errorcode so the canonical name wins there.
print("alias:", errno.EWOULDBLOCK == errno.EAGAIN)
print("alias canonical:", errno.errorcode[errno.EAGAIN] == "EAGAIN")

# a couple of common names simply have to be present.
print("has ENOSYS:", hasattr(errno, "ENOSYS"))
print("has ETIMEDOUT:", hasattr(errno, "ETIMEDOUT"))
print("has ECONNRESET:", hasattr(errno, "ECONNRESET"))
