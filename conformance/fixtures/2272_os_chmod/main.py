import os

# os.chmod sets the exact permission bits, observable through os.stat, for a str
# path and a bytes one. The file lives in the process's own working directory,
# which the harness gives each fixture fresh.
name = "probe.txt"
with open(name, "w") as fh:
    fh.write("x")

os.chmod(name, 0o644)
print("644:", oct(os.stat(name).st_mode & 0o777))
os.chmod(name, 0o600)
print("600:", oct(os.stat(name).st_mode & 0o777))
os.chmod(os.fsencode(name), 0o640)
print("640 (bytes):", oct(os.stat(name).st_mode & 0o777))


def show(label, fn):
    try:
        fn()
    except Exception as e:
        print(label, type(e).__name__, str(e))
    else:
        print(label, "no-error")


# A non-int mode, a missing mode, and an embedded NUL are the argument errors
# CPython raises, with the same messages on every platform.
show("mode-type", lambda: os.chmod(name, "x"))
show("missing-mode", lambda: os.chmod(name))
show("null-str", lambda: os.chmod("a\x00b", 0o644))
show("null-bytes", lambda: os.chmod(b"a\x00b", 0o644))

os.remove(name)
print("removed:", not os.path.exists(name))
