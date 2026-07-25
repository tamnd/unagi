import cProfile
import _lsprof

# _lsprof.Profiler is a real, subclassable type, so cProfile.Profile subclasses
# it through the ordinary class machinery.
print("imported")
print(isinstance(_lsprof.Profiler, type))
print(issubclass(cProfile.Profile, _lsprof.Profiler))

# A profiler can be constructed and driven. The C profiler installs a per-call
# hook on the bytecode eval loop; a compiled program has no such loop, so enable
# and disable are inert here. getstats returns a list either way, and runcall
# runs the target and returns its result, both host-invariant. The recorded entry
# count is not printed since it is meaningfully non-empty only under CPython's
# interpreter.
p = cProfile.Profile()
p.enable()
total = 0
for i in range(100):
    total += i
p.disable()
print(total)
print(type(p.getstats()).__name__)


def work(a, b):
    return a * b + 1


pr = cProfile.Profile()
print(pr.runcall(work, 6, 7))
print(p.getstats() is not None)
