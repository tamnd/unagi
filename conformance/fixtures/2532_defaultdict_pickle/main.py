# defaultdict pickles through the five-tuple reduction CPython's defaultdict
# returns: (defaultdict_type, args, None, None, items_iterator), where args is
# (default_factory,) when a factory is set and the empty tuple otherwise. The
# reconstructor rebuilds the factory before the item iterator's pairs are set
# back, so the mapping comes back with its factory live, its pairs in insertion
# order, and a missing key still materializing. The factory (a builtin type)
# pickles as a builtins global, the piece that unblocked this.
import pickle
from collections import defaultdict


def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as e:
        print(label, type(e).__name__, ":", e)


def rt(x, p):
    return pickle.loads(pickle.dumps(x, p))


dd = defaultdict(int, {"a": 1, "b": 2, "c": 3})

# Reduction shape.
show("reduce len", lambda: len(dd.__reduce__()))
show("reduce callable", lambda: dd.__reduce__()[0] is defaultdict)
show("reduce args", lambda: dd.__reduce__()[1])
show("reduce s2 s3", lambda: (dd.__reduce__()[2], dd.__reduce__()[3]))
show("reduce items", lambda: dict(dd.__reduce__()[4]))

# Round-trip across the binary protocols, with the factory and order intact.
for p in (2, 3, 4, 5):
    show("rt %d" % p, lambda pp=p: rt(dd, pp))
    show("factory %d" % p, lambda pp=p: rt(dd, pp).default_factory is int)
    show("order %d" % p, lambda pp=p: list(rt(dd, pp).items()))

# No factory reduces through the empty args tuple: default_factory stays None.
nd = defaultdict()
nd["x"] = 9
show("nofactory args", lambda: nd.__reduce__()[1])
show("nofactory rt", lambda: rt(nd, 5))
show("nofactory df", lambda: rt(nd, 5).default_factory)

# Other builtin-type factories round-trip to the same singleton.
show("list-factory", lambda: rt(defaultdict(list, {"x": [1, 2]}), 5))
show("set-factory df", lambda: rt(defaultdict(set), 5).default_factory is set)
show("dict-factory df", lambda: rt(defaultdict(dict, {"k": {"i": 1}}), 5).default_factory is dict)

# The restored factory materializes a missing key.
show("factory works", lambda: rt(defaultdict(int, {"a": 1}), 5)["zzz"])
show("factory list works", lambda: rt(defaultdict(list), 5)["new"])

# A nested mutable is copied, not shared, so a later mutation of the source does
# not reach the unpickled value.
src = defaultdict(list, {"k": [1]})
back = rt(src, 5)
src["k"].append(9)
print("nested-indep", back["k"])

# A shared reference to one defaultdict stays shared through the pickle.
d = defaultdict(int, {"a": 1})
show("shared", lambda: (lambda o: o[0] is o[1])(rt([d, d], 5)))

# Arity errors match the object-inherited wording.
show("reduce_ex 0-arg", lambda: dd.__reduce_ex__())
show("reduce 1-arg", lambda: dd.__reduce__(2))

# The reductions read back as bound-method values and hasattr agrees.
show("hasattr reduce", lambda: hasattr(dd, "__reduce_ex__"))
show("bound reduce", lambda: callable(dd.__reduce_ex__))
