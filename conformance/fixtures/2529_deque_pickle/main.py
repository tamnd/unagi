import pickle
from collections import deque


def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as e:
        print(label, type(e).__name__, ":", e)


def rt(x, proto):
    return pickle.loads(pickle.dumps(x, proto))


# A deque survives a round-trip at every binary protocol, elements and maxlen
# intact, since its reduction replays the elements as appends.
for proto in (2, 3, 4, 5):
    show("bounded %d" % proto, lambda p=proto: rt(deque([1, 2, 3], maxlen=5), p))
    show("bounded maxlen %d" % proto, lambda p=proto: rt(deque([1, 2, 3], maxlen=5), p).maxlen)

# An unbounded deque comes back unbounded.
show("unbounded", lambda: rt(deque([1, 2, 3]), 5))
show("unbounded maxlen", lambda: rt(deque([1, 2, 3]), 5).maxlen)

# An empty bounded deque keeps its maxlen with no elements.
show("empty", lambda: rt(deque([], maxlen=2), 5))
show("empty maxlen", lambda: rt(deque([], maxlen=2), 5).maxlen)

# Nested mutables ride through and the round-trip is a fresh, independent deque.
src = deque([[1, 2], [3, 4]], maxlen=3)
back = rt(src, 5)
print("nested", back)
src[0].append(99)
print("nested independent", back[0])

# A string deque and a mixed deque both round-trip.
show("chars", lambda: rt(deque("abc"), 5))
show("mixed", lambda: rt(deque([1, "two", 3.0, (4, 5)]), 5))

# A deque referenced twice memoizes, so the two come back the same object.
d = deque([1, 2])
pair = rt([d, d], 5)
print("shared", pair[0] is pair[1])

# The dumped bytes reload to an equal deque across a fresh loads too.
blob = pickle.dumps(deque([10, 20, 30], maxlen=4), 5)
print("reload", pickle.loads(blob), pickle.loads(blob).maxlen)
