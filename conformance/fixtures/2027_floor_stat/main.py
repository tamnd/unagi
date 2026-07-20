# _stat is the accelerator stat.py runs on: it names the file-mode bits and the
# S_IS*/S_IMODE/S_IFMT/filemode helpers that read a stat_result's st_mode. The
# mode bits are POSIX-universal, identical on the oracle host and the corpus
# host, so this floor asserts them by value along with the classifier results.
# This is sub-slice 6b of the posix arc (Spec 2076 stdlib S0_posix_arc.md).
import _stat

# the type bits and permission bits are the same octal values everywhere.
print("S_IFDIR", oct(_stat.S_IFDIR))
print("S_IFREG", oct(_stat.S_IFREG))
print("S_IFLNK", oct(_stat.S_IFLNK))
print("S_ISUID", oct(_stat.S_ISUID))
print("S_ISVTX", oct(_stat.S_ISVTX))
print("S_IRWXU", oct(_stat.S_IRWXU))
print("S_IFDOOR", _stat.S_IFDOOR)

# the stat_result tuple indices.
print("indices", _stat.ST_MODE, _stat.ST_INO, _stat.ST_SIZE, _stat.ST_CTIME)

# S_IMODE keeps the permission bits, S_IFMT keeps the type bits.
print("S_IMODE", oct(_stat.S_IMODE(0o100644)))
print("S_IFMT", oct(_stat.S_IFMT(0o100644)))

# the type predicates classify a mode by its type bits.
print("ISDIR", _stat.S_ISDIR(0o040755), _stat.S_ISDIR(0o100644))
print("ISREG", _stat.S_ISREG(0o100644), _stat.S_ISREG(0o040755))
print("ISLNK", _stat.S_ISLNK(0o120777))
print("ISCHR", _stat.S_ISCHR(0o020600))
print("ISFIFO", _stat.S_ISFIFO(0o010644))
print("ISSOCK", _stat.S_ISSOCK(0o140755))
print("ISDOOR", _stat.S_ISDOOR(0o100644))

# filemode renders the ls-style string, including the setuid/setgid/sticky forms.
print("file", _stat.filemode(0o100644))
print("dir", _stat.filemode(0o040755))
print("link", _stat.filemode(0o120777))
print("setuid", _stat.filemode(0o104755))
print("setgid-nox", _stat.filemode(0o102644))
print("sticky", _stat.filemode(0o041777))
print("unknown-type", _stat.filemode(0o000644))

# a non-integer mode is a TypeError.
try:
    _stat.S_IMODE("nope")
except TypeError:
    print("S_IMODE type error")

# stat.py runs on the accelerator: its classifiers match and it still carries
# the portable flags _stat does not define.
import stat
print("stat ISDIR", stat.S_ISDIR(0o040000))
print("stat filemode", stat.filemode(0o100755))
print("stat UF_APPEND", oct(stat.UF_APPEND))
