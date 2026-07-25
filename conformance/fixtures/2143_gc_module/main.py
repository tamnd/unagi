# The gc module is the interface to the cyclic garbage collector. unagi runs on
# Go's collector, so there is no separate generational collector to drive, but
# the surface the stdlib touches still has to answer. timeit disables the
# collector around a timing loop and trace calls gc.get_referrers, and import
# trace failed outright without the module.
#
# The concrete counts a live collector reports are host specific (get_count,
# get_objects, get_threshold values, get_debug), so these check the stable
# invariants: the enable flag toggles and reads back, collect reports a
# non-negative int, the shape queries return the right container types, and
# is_tracked splits containers from atomics the way both interpreters do.
import gc

# The enabled flag is a real toggle a program can read back.
gc.enable()
print(gc.isenabled())
gc.disable()
print(gc.isenabled())
gc.enable()
print(gc.isenabled())

# collect reports a non-negative count of unreachable objects.
c = gc.collect()
print(isinstance(c, int) and c >= 0)

# The generation counts and thresholds are three-int tuples.
counts = gc.get_count()
print(isinstance(counts, tuple) and len(counts) == 3 and all(isinstance(n, int) for n in counts))
thresholds = gc.get_threshold()
print(isinstance(thresholds, tuple) and len(thresholds) == 3 and all(isinstance(n, int) for n in thresholds))

# The object-graph queries return lists.
print(isinstance(gc.get_objects(), list))
print(isinstance(gc.get_referrers(gc), list))
print(isinstance(gc.get_referents(gc), list))

# is_tracked splits containers from atomic values.
print(gc.is_tracked([]))
print(gc.is_tracked({}))
print(gc.is_tracked(5))
print(gc.is_tracked("s"))

# garbage is the live uncollectable list.
print(isinstance(gc.garbage, list))

# The setters return None and accept the documented arguments.
print(gc.set_threshold(700, 10, 10) is None)
print(gc.set_debug(0) is None)
print(isinstance(gc.get_debug(), int))

# The freeze surface answers too.
print(gc.freeze() is None)
print(gc.unfreeze() is None)
print(isinstance(gc.get_freeze_count(), int))
