import os

# Size and type: urandom returns exactly the requested number of bytes.
b = os.urandom(16)
print(type(b).__name__, len(b))
print(len(os.urandom(0)), len(os.urandom(1)), len(os.urandom(255)))

# Empty request is an empty bytes, not an error.
print(os.urandom(0) == b"")

# The bytes are drawn fresh each call, so repeated 16-byte draws differ.
draws = {os.urandom(16) for _ in range(50)}
print(len(draws) == 50)

# A negative size is a ValueError, a non-integer a TypeError, matching CPython.
try:
    os.urandom(-1)
except ValueError as e:
    print("ValueError", e)
try:
    os.urandom("x")
except TypeError as e:
    print("TypeError", e)
try:
    os.urandom(1.5)
except TypeError as e:
    print("TypeError float", e)

# os.urandom is the same object re-exported from posix.
import posix
print(os.urandom is posix.urandom)
