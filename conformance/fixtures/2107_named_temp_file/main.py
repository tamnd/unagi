# _io.FileIO.name is a writable attribute in CPython, not a read-only property,
# and tempfile.NamedTemporaryFile overwrites it with the temp file's path
# (tempfile.py:609 raw.name = name). With the setter in place that assignment
# works, so NamedTemporaryFile runs end to end.
import io
import os
import tempfile

# The raw setter stores a value the getter reads back. The initial value is the
# file descriptor, which is not portable, so only the assigned value is printed.
fd, seed = tempfile.mkstemp()
raw = io.FileIO(fd, "w", closefd=False)
raw.name = "custom-name"
print(repr(raw.name))
os.close(fd)
os.unlink(seed)

# NamedTemporaryFile exposes a str name carrying the requested suffix, the file
# exists while open, its bytes read back, and delete=False leaves it for an
# explicit unlink.
with tempfile.NamedTemporaryFile(delete=False, suffix=".txt", prefix="unagi_") as tf:
    tf.write(b"payload")
    name = tf.name

print(type(name).__name__)
print(name.endswith(".txt"))
print(os.path.basename(name).startswith("unagi_"))
print(os.path.exists(name))
with open(name, "rb") as f:
    print(f.read())
os.unlink(name)
print(os.path.exists(name))
