import mmap
import os
import tempfile

# mmap is the memory-mapped-file accelerator. The fixture maps a real file and a
# file-less anonymous region and exercises the sequence protocol (len, index,
# slice, slice assignment), the read/write cursor, find/rfind, the access modes
# and the context manager. The PAGESIZE value is host-specific, so only its
# positivity is checked, not the number.

fd, path = tempfile.mkstemp()
try:
    os.write(fd, b"Hello World\nsecond line\n")
    os.fsync(fd)
    with mmap.mmap(fd, 0) as mm:
        print("len:", len(mm))
        print("index:", mm[0])
        print("slice:", mm[0:5])
        print("step slice:", mm[0:11:2])
        print("find:", mm.find(b"World"))
        print("rfind:", mm.rfind(b"line"))
        mm[0] = ord("J")
        mm[6:11] = b"WORLD"
        print("after edits:", mm[0:11])
        mm.seek(0)
        print("read:", mm.read(5))
        print("tell:", mm.tell())
        print("read_byte:", mm.read_byte())
        mm.seek(0)
        print("readline:", mm.readline())
        print("size:", mm.size())
    print("closed after with:", mm.closed)

    # ACCESS_READ maps read-only: a write raises TypeError.
    ro = mmap.mmap(fd, 0, access=mmap.ACCESS_READ)
    print("readonly slice:", ro[0:5])
    try:
        ro[0] = 65
    except TypeError as e:
        print("readonly write:", str(e))
    ro.close()

    # ACCESS_COPY is copy-on-write: the edit is private and never reaches the file.
    cow = mmap.mmap(fd, 0, access=mmap.ACCESS_COPY)
    cow[0] = ord("Z")
    print("copy local:", cow[0:5])
    cow.close()
    with open(path, "rb") as f:
        print("file first byte:", f.read(1))
finally:
    os.close(fd)
    os.unlink(path)

# Anonymous mapping: no file, must be given a size.
anon = mmap.mmap(-1, 16)
anon.write(b"abc")
anon.seek(0)
print("anon read:", anon.read(3))
anon.close()

print("access modes:", mmap.ACCESS_READ, mmap.ACCESS_WRITE, mmap.ACCESS_COPY, mmap.ACCESS_DEFAULT)
print("pagesize positive:", mmap.PAGESIZE > 0)
print("done")
