# posix is the syscall accelerator os.py runs on. This floor drives the leaf
# surface the skeleton stands up: the error alias, the access-mode constants,
# the open flags, environ and the fd-free calls. The open-flag and pid values
# are platform-specific, so this asserts only what is identical across the
# oracle host and the corpus host (the access constants, which are POSIX
# universal) plus structural properties (types, distinctness, round-trips).
# This is sub-slice 6c of the posix arc (Spec 2076 stdlib S0_posix_arc.md).
import posix

# error is posix's name for OSError.
print("error is OSError:", posix.error is OSError)

# the access-mode constants are the same on every POSIX host.
print("access:", posix.F_OK, posix.R_OK, posix.W_OK, posix.X_OK)

# the open flags are platform-specific, so only their shape is asserted.
flags = [posix.O_RDONLY, posix.O_WRONLY, posix.O_RDWR, posix.O_APPEND,
         posix.O_CREAT, posix.O_EXCL, posix.O_TRUNC, posix.O_NONBLOCK,
         posix.O_CLOEXEC, posix.O_DIRECTORY, posix.O_NOFOLLOW]
print("flags ints:", all(isinstance(f, int) for f in flags))
print("access modes distinct:", posix.O_RDONLY != posix.O_WRONLY != posix.O_RDWR)

# _have_functions gates fd-aware call variants; the skeleton advertises none.
print("have_functions list:", isinstance(posix._have_functions, list))

# the process ids are positive; the values themselves vary per run.
print("pids positive:", posix.getpid() > 0 and posix.getppid() > 0)

# getcwd returns str, getcwdb the same path as bytes.
print("cwd str:", isinstance(posix.getcwd(), str))
print("cwdb bytes:", isinstance(posix.getcwdb(), bytes))
print("cwd matches cwdb:", posix.getcwd() == posix.getcwdb().decode())

# strerror returns a non-empty message; the wording is platform-specific.
print("strerror str:", isinstance(posix.strerror(2), str) and len(posix.strerror(2)) > 0)

# umask sets and returns the previous mask; set a known value, read it back,
# then restore, so the asserted value does not depend on the ambient mask.
prev = posix.umask(0o22)
cur = posix.umask(prev)
print("umask roundtrip:", cur == 0o22)

# listdir returns the directory names as a list of str.
names = posix.listdir(".")
print("listdir list of str:", isinstance(names, list) and all(isinstance(n, str) for n in names))

# environ is a bytes->bytes dict; _create_environ hands back a fresh one.
k = next(iter(posix.environ))
print("environ bytes:", isinstance(k, bytes) and isinstance(posix.environ[k], bytes))
fresh = posix._create_environ()
print("create_environ dict:", isinstance(fresh, dict) and fresh is not posix.environ)

# a bad argument count is a TypeError, not a crash.
try:
    posix.getpid(1)
except TypeError:
    print("getpid arg error")
