import collections


def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# A deque method reads back as a bound built-in method, so d.append is a value
# that carries the deque through __self__, names __qualname__ after deque and
# keeps __name__ bare, then calls the same as the fused d.append(x). Before this
# a read raised AttributeError even though the fused call worked.
d = collections.deque([1, 2, 3])
print("== a deque method reads back as a bound method ==")
show("hasattr(d, 'append')", lambda: hasattr(d, "append"))
show("d.append.__name__", lambda: d.append.__name__)
show("d.append.__qualname__", lambda: d.append.__qualname__)
show("d.append.__self__ is d", lambda: d.append.__self__ is d)
show("d.appendleft.__qualname__", lambda: d.appendleft.__qualname__)
show("d.rotate.__qualname__", lambda: d.rotate.__qualname__)
show("d.popleft.__self__ is d", lambda: d.popleft.__self__ is d)
show("d.__reversed__.__qualname__", lambda: d.__reversed__.__qualname__)
show("type(d.append).__name__", lambda: type(d.append).__name__)

print("== the read binds and calls the same as a fused call ==")
d2 = collections.deque([1, 2, 3])
push = d2.append
show("read append then call", lambda: (push(4), list(d2))[1])
pushleft = d2.appendleft
show("read appendleft then call", lambda: (pushleft(0), list(d2))[1])
show("read count then call", lambda: collections.deque([5, 5, 6]).count(5))
show("read index then call", lambda: collections.deque(["a", "b", "c"]).index("b"))

print("== a missing method still reports the deque attribute error ==")
show("d.nope", lambda: d.nope)
show("hasattr(d, 'sort')", lambda: hasattr(d, "sort"))
