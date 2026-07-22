# pwd backs posixpath.expanduser for the ~user form. root is the one account
# present on every POSIX host with a fixed uid of 0, so the assertions read it
# and check only host-invariant facts: its name, uid and gid, that the home is
# absolute, and that the name and uid lookups agree. The actual home path,
# shell and gecos differ across hosts, so none of those values are printed.
import pwd
import os.path

r = pwd.getpwnam("root")
print("type", type(r).__name__)
print("len", len(r))
print("pw_name", r.pw_name)
print("pw_uid", r.pw_uid)
print("pw_gid", r.pw_gid)
print("home_abs", r.pw_dir.startswith("/"))
print("is_tuple", isinstance(r, tuple))
print("index0", r[0])
print("index2", r[2])

byuid = pwd.getpwuid(0)
print("byuid_name", byuid.pw_name)
print("byuid_home_matches", byuid.pw_dir == r.pw_dir)

# expanduser('~root') expands to root's home, which is absolute and no longer
# starts with a tilde. An unknown user is left unchanged.
exp = os.path.expanduser("~root")
print("expand_abs", exp.startswith("/"))
print("expand_changed", exp != "~root")
print("expand_matches_home", exp == r.pw_dir)
print("expand_unknown", os.path.expanduser("~no_such_user_xyzzy"))

# A missing name or uid raises KeyError.
try:
    pwd.getpwnam("no_such_user_xyzzy")
except KeyError:
    print("getpwnam_missing_raises")
try:
    pwd.getpwuid(4294967295)
except KeyError:
    print("getpwuid_missing_raises")
