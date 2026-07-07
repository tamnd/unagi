# stat is a pure-Python floor module: it compiles from the bundled pinned
# CPython source, not from a Go accelerator. Its only reach for the C _stat
# module sits behind a guarded import, so it runs as ordinary compiled input.
# This drives the mode constants, the type predicates, the bit extractors, and
# filemode, the pieces the rest of the floor and the test suite lean on.
import stat

# The file-type predicates over the matching type bits.
print(stat.S_ISDIR(stat.S_IFDIR))
print(stat.S_ISREG(stat.S_IFREG))
print(stat.S_ISLNK(stat.S_IFLNK))
print(stat.S_ISCHR(stat.S_IFCHR))
print(stat.S_ISBLK(stat.S_IFBLK))
print(stat.S_ISFIFO(stat.S_IFIFO))
print(stat.S_ISSOCK(stat.S_IFSOCK))

# A regular file's mode is not a directory, and the reverse.
print(stat.S_ISDIR(0o100644))
print(stat.S_ISREG(0o040755))

# S_IFMT masks out the permission bits, S_IMODE keeps them.
print(oct(stat.S_IFMT(0o100644)))
print(oct(stat.S_IFMT(0o040755)))
print(oct(stat.S_IMODE(0o100644)))
print(oct(stat.S_IMODE(0o104755)))

# The permission bit constants carry their POSIX values.
print(oct(stat.S_IRWXU), oct(stat.S_IRUSR), oct(stat.S_IWUSR), oct(stat.S_IXUSR))
print(oct(stat.S_ISUID), oct(stat.S_ISGID), oct(stat.S_ISVTX))

# filemode spells a mode the way ls does, across the type letters and the
# special permission bits.
print(stat.filemode(0o100644))
print(stat.filemode(0o040755))
print(stat.filemode(0o120777))
print(stat.filemode(0o104755))
print(stat.filemode(0o042755))
print(stat.filemode(0o106644))
print(stat.filemode(0o010644))

# The file-type flags read for a filesystem walk.
print(oct(stat.SF_IMMUTABLE))
print(stat.UF_NODUMP)
