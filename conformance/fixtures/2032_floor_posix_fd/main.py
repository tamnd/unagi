# posix fd I/O: open returns a small int fd that read/write/lseek/close and the
# rest work on. This runs in the fixture's throwaway cwd, so it creates a file,
# rounds bytes through it, and prints only host-invariant results: the fd numbers
# themselves are not printed since the lowest free descriptor is not portable.
# whence values 0/1/2 are SEEK_SET/CUR/END, hardcoded here because os.py (which
# defines the SEEK_* names) is not up yet.
import posix

fd = posix.open("data.bin", posix.O_CREAT | posix.O_WRONLY | posix.O_TRUNC, 0o644)
print("write", posix.write(fd, b"hello world"))
print("isatty", posix.isatty(fd))
posix.close(fd)

fd = posix.open("data.bin", posix.O_RDONLY)
print("seek", posix.lseek(fd, 6, 0))
print("read", posix.read(fd, 5))
print("tell", posix.lseek(fd, 0, 1))
print("rewind", posix.lseek(fd, 0, 0))
print("readall", posix.read(fd, 100))
print("ateof", posix.read(fd, 100))
print("fstat_size", posix.fstat(fd).st_size)
posix.close(fd)

# truncate then re-read
fd = posix.open("data.bin", posix.O_RDWR)
posix.ftruncate(fd, 5)
posix.fsync(fd)
posix.close(fd)
fd = posix.open("data.bin", posix.O_RDONLY)
print("after_trunc", posix.read(fd, 100))
fd2 = posix.dup(fd)
print("dup_distinct", fd != fd2)
posix.close(fd)
posix.close(fd2)

# pipe plus dup2 onto a chosen descriptor
r, w = posix.pipe()
target = posix.dup2(w, 15)
print("dup2_target", target)
posix.write(target, b"pi")
print("pipe_read", posix.read(r, 2))
posix.close(r)
posix.close(w)
posix.close(target)
