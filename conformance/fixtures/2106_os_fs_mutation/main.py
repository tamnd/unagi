# os grew the filesystem-mutation calls unlink/remove/mkdir/rmdir/rename, which
# tempfile binds at import (unlink is a default argument), and sys.audit is a
# no-op with no hooks installed, so tempfile imports and its directory helpers
# run. The fixture works inside a scratch directory whose random name it never
# prints, so only the deterministic results show.
import os
import tempfile

root = tempfile.mkdtemp()

# mkdir makes one directory; the mode is masked by umask so only checking that
# it exists as a directory stays portable.
sub = os.path.join(root, "sub")
os.mkdir(sub)
print(os.path.isdir(sub))

# A written file can be read back, sized, renamed and removed.
a = os.path.join(sub, "a.txt")
with open(a, "w") as f:
    f.write("hello unagi")
print(os.path.exists(a), os.path.getsize(a))
with open(a) as f:
    print(f.read())

b = os.path.join(sub, "b.txt")
os.rename(a, b)
print(os.path.exists(a), os.path.exists(b))

# remove is the same call as unlink under a second name.
os.remove(b)
print(os.path.exists(b))

# rmdir takes an empty directory.
os.rmdir(sub)
print(os.path.exists(sub))
os.rmdir(root)
print(os.path.exists(root))

# mkstemp opens a real fd in the temp dir; the bytes written come back at the
# recorded size, then the file is removed.
fd, path = tempfile.mkstemp()
os.write(fd, b"abc")
os.close(fd)
print(os.path.getsize(path))
os.unlink(path)
print(os.path.exists(path))

# gettempdir hands back a str, and the audit hook machinery is present: audit
# is a no-op returning None with no hook installed.
import sys

print(type(tempfile.gettempdir()).__name__)
print(sys.audit("tempfile.mkdtemp", None) is None)
