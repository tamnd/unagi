# posix.stat / lstat / access floor: the stat_result structseq shape.
# Everything asserted here is identical on darwin and Linux, so no
# platform-divergent numbers (mode bits, sizes, field counts past the
# common set) appear. The cwd is a directory that always exists, which
# lets the fixture stand up without creating a file (posix has no write
# until the fd slice).
import posix
import stat

d = posix.getcwd()
st = posix.stat(d)

print("istuple", isinstance(st, tuple))
print("len", len(st))
print("isdir", stat.S_ISDIR(st.st_mode))
print("isreg", stat.S_ISREG(st.st_mode))

# The sequence carries the int seconds; the named attribute is the float.
print("seq7_is_int", isinstance(st[7], int))
print("seq7_matches_atime", st[7] == int(st.st_atime))
print("atime_is_float", isinstance(st.st_atime, float))
print("mtime_is_float", isinstance(st.st_mtime, float))
print("ctime_is_float", isinstance(st.st_ctime, float))

# The nanosecond attributes are exact ints, distinct from the tuple slot.
print("atime_ns_is_int", isinstance(st.st_atime_ns, int))
print("mtime_ns_is_int", isinstance(st.st_mtime_ns, int))
print("ctime_ns_is_int", isinstance(st.st_ctime_ns, int))
print("blksize_is_int", isinstance(st.st_blksize, int))
print("blocks_is_int", isinstance(st.st_blocks, int))
print("rdev_is_int", isinstance(st.st_rdev, int))

print("n_seq", st.n_sequence_fields)
print("n_unnamed", st.n_unnamed_fields)

lst = posix.lstat(d)
print("lstat_isdir", stat.S_ISDIR(lst.st_mode))
print("lstat_same_mode", lst.st_mode == st.st_mode)

print("access_ok", posix.access(d, posix.F_OK))
print("access_read", posix.access(d, posix.R_OK))
print("access_missing", posix.access(d + "/no_such_entry_zzz", posix.F_OK))

print("stat_result_name", posix.stat_result.__name__)
print("stat_result_nseq", posix.stat_result.n_sequence_fields)
print("stat_result_nunnamed", posix.stat_result.n_unnamed_fields)
