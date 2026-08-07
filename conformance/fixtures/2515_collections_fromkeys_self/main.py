import collections


def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# fromkeys is a classmethod, so reading it off a collections dict instance names
# the type through __self__ the way CPython inherits it onto an instance, rather
# than binding the instance the way an ordinary method read does. __qualname__
# qualifies to the short type name and __name__ stays bare.
print("== a collections dict names its type through fromkeys ==")
show("OrderedDict().fromkeys.__self__", lambda: collections.OrderedDict().fromkeys.__self__)
show("OrderedDict().fromkeys.__qualname__", lambda: collections.OrderedDict().fromkeys.__qualname__)
show("OrderedDict().fromkeys.__name__", lambda: collections.OrderedDict().fromkeys.__name__)
show("defaultdict(int).fromkeys.__self__", lambda: collections.defaultdict(int).fromkeys.__self__)
show("defaultdict(int).fromkeys.__qualname__", lambda: collections.defaultdict(int).fromkeys.__qualname__)
show("defaultdict(int).fromkeys.__name__", lambda: collections.defaultdict(int).fromkeys.__name__)

print("== an ordinary method still binds the instance ==")
show("OrderedDict().get.__self__ is the instance", lambda: (lambda d: d.get.__self__ is d)(collections.OrderedDict([("a", 1)])))
show("defaultdict().get.__self__ is the instance", lambda: (lambda d: d.get.__self__ is d)(collections.defaultdict(int)))

print("== the classmethod still builds its kind ==")
show("OrderedDict().fromkeys(['a', 'b'], 0)", lambda: collections.OrderedDict().fromkeys(["a", "b"], 0))
show("defaultdict(int).fromkeys(['x'], 1)", lambda: collections.defaultdict(int).fromkeys(["x"], 1))
bound = collections.OrderedDict().fromkeys
show("bound read then call", lambda: bound(["p", "q"], 9))
