# deque.__copy__ makes an independent shallow copy that preserves the maxlen,
# the hook copy.copy reaches through getattr(type(d), "__copy__"). Before it,
# both the bound d.__copy__() read and copy.copy(d) raised.
def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as e:
        print(label, type(e).__name__, ":", e)


from collections import deque
import copy

d = deque([1, 2, 3], maxlen=5)
show("hasattr __copy__", lambda: hasattr(d, "__copy__"))
show("d.__copy__()", lambda: d.__copy__())
show("copy.copy(d)", lambda: copy.copy(d))
show("copy.copy unbounded maxlen", lambda: copy.copy(deque([1, 2, 3])).maxlen)
show("empty deque copy", lambda: copy.copy(deque()))
show("type(d).__copy__(d)", lambda: list(type(d).__copy__(d)))

c = copy.copy(d)
c.append(9)
show("original unchanged", lambda: list(d))
show("copy has new item", lambda: list(c))
show("copy maxlen preserved", lambda: c.maxlen)
