import copy
from collections import deque


def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as e:
        print(label, type(e).__name__, ":", e)


# __reduce__ returns a 4-tuple: (type, args, state, elements iterator).
d = deque([1, 2, 3], maxlen=5)
r = d.__reduce__()
print("len", len(r))
print("type", r[0] is deque)
print("args", r[1])
print("state", r[2])
print("elems", list(r[3]))

# Unbounded deques reduce with empty args.
u = deque([1, 2, 3])
print("unbounded args", u.__reduce__()[1])

# Empty bounded deque still carries its maxlen.
e = deque([], maxlen=2)
print("empty bounded args", e.__reduce__()[1])

# __reduce_ex__ agrees with __reduce__ on shape.
x = deque([9, 8], maxlen=4)
print("reduce_ex len", len(x.__reduce_ex__(4)))
print("reduce_ex args", x.__reduce_ex__(2)[1])

# copy.deepcopy reduces through __reduce_ex__ and rebuilds an independent deque,
# including the maxlen and any nested mutables.
src = deque([[1, 2], [3, 4]], maxlen=3)
dc = copy.deepcopy(src)
print("deepcopy", dc)
print("deepcopy maxlen", dc.maxlen)
src[0].append(99)
print("deepcopy independent", dc[0])
print("deepcopy not same object", dc is src)

# copy.copy shares the shallow-copy path.
sc = copy.copy(deque([1, 2, 3], maxlen=5))
print("copy", sc, sc.maxlen)

# Arity guards match CPython's messages.
show("reduce extra arg", lambda: deque([1]).__reduce__(0))
show("reduce_ex no arg", lambda: deque([1]).__reduce_ex__())
show("reduce_ex extra arg", lambda: deque([1]).__reduce_ex__(2, 3))
