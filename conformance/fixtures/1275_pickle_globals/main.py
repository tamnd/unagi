import pickle

# A module-level function and a class object itself pickle as a bare global
# reference: the module and qualname go out as GLOBAL (protocol 2/3) or
# STACK_GLOBAL (protocol 4+), with no reduction. This is the pickle-by-qualified-
# name path multiprocessing uses to ship a spawn worker's target. The exact bytes
# are observable, so the module name, the qualname, and the memo puts must match
# CPython slot for slot.


def greet(name):
    return "hi " + name


class Point:
    def __init__(self, x, y):
        self.x = x
        self.y = y

    def __eq__(self, other):
        return isinstance(other, Point) and self.x == other.x and self.y == other.y


# A function and a class each pickle as their qualified-name reference across the
# binary protocols. The unpickled global is the very same object, so identity
# holds and the recovered function still calls.
for proto in (2, 3, 4, 5):
    fdata = pickle.dumps(greet, protocol=proto)
    cdata = pickle.dumps(Point, protocol=proto)
    print("greet", proto, fdata.hex(), pickle.loads(fdata) is greet)
    print("Point", proto, cdata.hex(), pickle.loads(cdata) is Point)

# The recovered function runs, and the recovered class instantiates.
print("call:", pickle.loads(pickle.dumps(greet))("world"))
rt_cls = pickle.loads(pickle.dumps(Point))
print("build:", rt_cls(2, 3) == Point(2, 3))

# The same global referenced twice in a container is written once and fetched
# back from the memo, so both slots recover the identical object.
pair = pickle.dumps([greet, greet])
pb = pickle.loads(pair)
print("shared fn:", pair.hex(), pb[0] is pb[1], pb[0] is greet)

# A class pickled directly and as the class of an instance in the same pickle
# shares one global reference: the instance's NEWOBJ names the class, and the
# bare class beside it fetches that same memo entry.
mixed = pickle.dumps((Point, Point(5, 6)))
mb = pickle.loads(mixed)
print("mixed:", mixed.hex(), mb[0] is Point, mb[1] == Point(5, 6))
