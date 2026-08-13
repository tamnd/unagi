# CPython refuses a callable it cannot reference by qualified name with one of
# two messages, both formatted with the object's repr. A qualname carrying a
# <locals> path segment (a nested def or class, a comprehension inside one) is a
# "local object"; a top-level lambda or generator expression, whose angle-bracket
# name is not a <locals> segment, "is not found" at its module path instead. A
# genuine module-level def or class stays picklable. This checks the wording and
# the still-working reachable path across protocols.
import pickle


def show(label, fn):
    try:
        r = fn()
        print(label, r)
    except Exception as e:
        print(label, type(e).__name__, ":", e)


# Top-level lambda: qualname <lambda>, no <locals> segment, so "not found".
lam = lambda x: x + 1


# Nested def and nested class: qualname carries <locals>, so "local object".
def outer():
    def inner():
        return 1

    return inner


nested_fn = outer()


def make_class():
    class Inner:
        pass

    return Inner


local_class = make_class()


# A comprehension nested in a function also carries <locals>.
def with_comp():
    return [(lambda: i) for i in range(1)][0]


nested_comp = with_comp()


# A genuine module-level def and class must still pickle and round-trip.
def top_fn(x):
    return x * 2


class TopClass:
    def __init__(self, v):
        self.v = v

    def __eq__(self, other):
        return isinstance(other, TopClass) and other.v == self.v


for proto in (2, 4, 5):
    show("lambda p%d" % proto, lambda proto=proto: pickle.dumps(lam, proto))
    show("nested p%d" % proto, lambda proto=proto: pickle.dumps(nested_fn, proto))
    show("localcls p%d" % proto, lambda proto=proto: pickle.dumps(local_class, proto))
    show("comp p%d" % proto, lambda proto=proto: pickle.dumps(nested_comp, proto))
    # The reachable global path is unaffected.
    show("topfn rt p%d" % proto,
         lambda proto=proto: pickle.loads(pickle.dumps(top_fn, proto)) is top_fn)
    show("topcls rt p%d" % proto,
         lambda proto=proto: pickle.loads(pickle.dumps(TopClass, proto)) is TopClass)
    show("inst rt p%d" % proto,
         lambda proto=proto: pickle.loads(pickle.dumps(TopClass(7), proto)) == TopClass(7))
