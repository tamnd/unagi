import resource

# resource is the process-resource accelerator test.support caps core dumps,
# file descriptors and address space with, and profilers read CPU time and peak
# RSS out of. The constant values are host-specific, so the fixture checks the
# shape of the surface and the behavior of a round-trip, not the raw numbers.

# The rusage and rlimit constants are present as ints.
for name in ["RUSAGE_SELF", "RUSAGE_CHILDREN", "RLIMIT_CPU", "RLIMIT_FSIZE",
             "RLIMIT_DATA", "RLIMIT_STACK", "RLIMIT_CORE", "RLIMIT_NOFILE",
             "RLIMIT_AS", "RLIMIT_NPROC", "RLIMIT_MEMLOCK", "RLIM_INFINITY"]:
    assert isinstance(getattr(resource, name), int), name
print("constants ok")

# resource.error is an alias for OSError.
print("error is OSError:", resource.error is OSError)

# getrusage returns a 16-field struct_rusage: two float time fields, int
# counters, addressable by name and by index alike.
ru = resource.getrusage(resource.RUSAGE_SELF)
print("len:", len(ru))
print("ru_utime float:", isinstance(ru.ru_utime, float))
print("ru_stime float:", isinstance(ru.ru_stime, float))
print("ru_maxrss int:", isinstance(ru.ru_maxrss, int))
print("ru_nvcsw int:", isinstance(ru.ru_nvcsw, int))
print("index==attr:", ru[0] == ru.ru_utime, ru[2] == ru.ru_maxrss, ru[15] == ru.ru_nivcsw)

# getrlimit returns a (soft, hard) pair of ints; setrlimit accepts it back
# unchanged, so a program can read a limit and restore it.
soft, hard = resource.getrlimit(resource.RLIMIT_NOFILE)
print("rlimit ints:", isinstance(soft, int), isinstance(hard, int))
resource.setrlimit(resource.RLIMIT_NOFILE, (soft, hard))
print("rlimit round-trip ok")

# getpagesize is a positive int.
print("pagesize positive:", resource.getpagesize() > 0)
print("done")
