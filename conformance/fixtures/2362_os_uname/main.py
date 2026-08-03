import os

u = os.uname()

# Structural checks only: the actual sysname/nodename/release/version/machine
# values are host specific, so the golden asserts the shape, not the contents.
print("type:", type(u).__name__)
print("len:", len(u))

fields = ["sysname", "nodename", "release", "version", "machine"]
print("fields:", [hasattr(u, name) for name in fields])

# Each named field equals its positional counterpart and is a str.
by_name = [getattr(u, name) for name in fields]
print("named == indexed:", by_name == list(u))
print("all str:", all(isinstance(v, str) for v in u))

# The named fields are also reachable as attributes on the result.
print("sysname is str:", isinstance(u.sysname, str))
print("sysname nonempty:", len(u.sysname) > 0)

# uname takes no arguments.
try:
    os.uname(1)
    print("argcheck: no error")
except TypeError:
    print("argcheck: TypeError")
