# The builtin open() and the vendored io module, end to end. FileIO is the raw
# fd stream, open() layers the buffered and text streams over it, and io.py now
# resolves from the vendored source on top of the _io accelerator. The fixture
# runs in its own throwaway directory, so it writes real files and reads them
# back, asserting only host-invariant facts (the contents it wrote, the stream
# shapes, the mode strings), never a path that embeds the temp directory.
import io

# io resolves to the vendored module now, not the old shim: it exposes the whole
# open()/FileIO surface and the SEEK_* and DEFAULT_BUFFER_SIZE constants.
print("io_name", io.__name__)
print("seek_set", io.SEEK_SET, io.SEEK_CUR, io.SEEK_END)
print("default_bufsize", io.DEFAULT_BUFFER_SIZE)
print("open_is_builtin_open", io.open is open)

# Text write then read back through the builtin open().
with open("notes.txt", "w") as f:
    print("write_count", f.write("alpha\nbeta\ngamma\n"))
    print("writable", f.writable(), "readable", f.readable())

with open("notes.txt") as f:
    print("read_all", repr(f.read()))

with open("notes.txt") as f:
    print("readlines", f.readlines())

with open("notes.txt") as f:
    print("first_line", repr(f.readline()))
    print("iter_rest", [line.rstrip("\n") for line in f])

# Append adds to the end.
with open("notes.txt", "a") as f:
    f.write("delta\n")
with open("notes.txt") as f:
    print("after_append", f.read().count("\n"))

# Binary round trip: bytes in, bytes out, and the mode string is binary.
with open("blob.bin", "wb") as f:
    f.write(b"\x00\x01\x02\xff")
with open("blob.bin", "rb") as f:
    data = f.read()
    print("bin_read", list(data))
    print("bin_type", type(data).__name__)

# Random access on a binary update stream: seek and tell over the raw file.
with open("blob.bin", "rb") as f:
    print("seek_end", f.seek(0, io.SEEK_END))
    f.seek(1)
    print("tell", f.tell())
    print("read_two", list(f.read(2)))

# A missing file for reading raises FileNotFoundError.
try:
    open("no_such_file_xyzzy.txt")
except FileNotFoundError:
    print("missing", "FileNotFoundError")

# An invalid mode raises ValueError.
try:
    open("notes.txt", "rw")
except ValueError:
    print("bad_mode", "ValueError")

# The in-memory streams the io module still re-exports.
s = io.StringIO()
s.write("in-")
s.write("memory")
print("stringio", s.getvalue())

b = io.BytesIO(b"bytes-io")
print("bytesio", b.read())

# open() with an explicit utf-8 encoding writes and reads non-ascii text.
with open("uni.txt", "w", encoding="utf-8") as f:
    f.write("café\n")
with open("uni.txt", "r", encoding="utf-8") as f:
    print("unicode", repr(f.read()))

print("done")
