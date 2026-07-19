# The _io accelerator is the C module vendored io.py imports from. This is the
# first slice of its surface: the module skeleton with UnsupportedOperation, the
# class io.py re-exports for an operation a stream does not support, plus
# DEFAULT_BUFFER_SIZE and the BlockingIOError re-export. The _IOBase family and
# the concrete streams are later sub-slices (Spec 2076 stdlib S0_io_arc.md).
import _io

# UnsupportedOperation reports itself as io.UnsupportedOperation and derives from
# both OSError and ValueError, so an except of either catches it.
print(repr(_io.UnsupportedOperation))
print(_io.UnsupportedOperation.__module__)
print(_io.UnsupportedOperation.__qualname__)
print(_io.UnsupportedOperation.__mro__)
print(issubclass(_io.UnsupportedOperation, OSError))
print(issubclass(_io.UnsupportedOperation, ValueError))

try:
    raise _io.UnsupportedOperation("seek")
except ValueError as e:
    print("ValueError:", e)
try:
    raise _io.UnsupportedOperation("fileno")
except OSError as e:
    print("OSError:", e)

# The constant and the re-exported builtin exception.
print(_io.DEFAULT_BUFFER_SIZE)
print(_io.BlockingIOError is BlockingIOError)

# The class is a stable singleton across imports.
import _io as again

print(again.UnsupportedOperation is _io.UnsupportedOperation)
