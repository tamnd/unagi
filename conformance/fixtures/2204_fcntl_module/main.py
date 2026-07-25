import fcntl
import os
import tempfile

# fcntl is the descriptor-control accelerator mailbox (lockf) and subprocess
# (fcntl, F_SETPIPE_SZ) build on. The fixture exercises the host-invariant
# surface and a controlled round-trip on a real descriptor; the flag values
# themselves are host-specific, so only their presence and behavior are checked.

# The lock operations are POSIX-fixed small integers on every host.
print("LOCK_SH/EX/UN/NB:", fcntl.LOCK_SH, fcntl.LOCK_EX, fcntl.LOCK_UN, fcntl.LOCK_NB)
# The descriptor-flag commands and FD_CLOEXEC are present as ints.
for name in ["F_GETFD", "F_SETFD", "F_GETFL", "F_SETFL", "FD_CLOEXEC"]:
    print(name, "is int:", isinstance(getattr(fcntl, name), int))

fd, path = tempfile.mkstemp()
try:
    # Control the close-on-exec flag rather than depend on the open default
    # (which is impl-specific): clear it, confirm clear, set it, confirm set.
    fcntl.fcntl(fd, fcntl.F_SETFD, 0)
    print("cleared:", bool(fcntl.fcntl(fd, fcntl.F_GETFD) & fcntl.FD_CLOEXEC))
    fcntl.fcntl(fd, fcntl.F_SETFD, fcntl.FD_CLOEXEC)
    print("set:", bool(fcntl.fcntl(fd, fcntl.F_GETFD) & fcntl.FD_CLOEXEC))

    # flock: whole-file BSD lock, exclusive then release.
    fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
    print("flock EX/UN:", end=" ")
    fcntl.flock(fd, fcntl.LOCK_UN)
    print("ok")

    # lockf: POSIX record lock, exclusive then release.
    fcntl.lockf(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
    print("lockf EX/UN:", end=" ")
    fcntl.lockf(fd, fcntl.LOCK_UN)
    print("ok")
finally:
    os.close(fd)
    os.unlink(path)

print("done")
