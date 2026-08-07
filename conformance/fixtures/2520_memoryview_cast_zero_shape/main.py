def show(label, fn):
    try:
        print(label, "=>", repr(fn()))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


# memoryview.cast(format, shape) reshaping a zero-length view is the
# cannot-cast-zeros TypeError, since a view with a zero dimension carries no
# elements to lay out under the new shape. The check runs ahead of the per-element
# shape validation, so even a shape with a bad element still reports the zero-view
# error first. A sized view keeps the per-element wording: a zero or negative
# dimension is the positive-elements ValueError and a non-integer element the
# integers TypeError.
print("== reshaping an empty view is the zero-view TypeError ==")
show("empty shape[0]", lambda: memoryview(b"").cast("B", shape=[0]))
show("empty shape[1]", lambda: memoryview(b"").cast("B", shape=[1]))
show("empty shape[0,1]", lambda: memoryview(b"").cast("B", shape=[0, 1]))
show("empty shape[1,0]", lambda: memoryview(b"").cast("B", shape=[1, 0]))
show("empty shape[]", lambda: memoryview(b"").cast("B", shape=[]))
show("empty shape['x']", lambda: memoryview(b"").cast("B", shape=["x"]))
show("empty shape[-1]", lambda: memoryview(b"").cast("B", shape=[-1]))
show("empty shape[2.0]", lambda: memoryview(b"").cast("B", shape=[2.0]))

print("== a sized view keeps the per-element wording ==")
show("6 shape[0,3]", lambda: memoryview(bytes(6)).cast("B", shape=[0, 3]))
show("6 shape[2,0]", lambda: memoryview(bytes(6)).cast("B", shape=[2, 0]))
show("6 shape[-1,3]", lambda: memoryview(bytes(6)).cast("B", shape=[-1, 3]))
show("6 shape[2,'x']", lambda: memoryview(bytes(6)).cast("B", shape=[2, "x"]))
show("6 shape[2.0,3]", lambda: memoryview(bytes(6)).cast("B", shape=[2.0, 3]))
show("6 shape[2,2]", lambda: memoryview(bytes(6)).cast("B", shape=[2, 2]))
show("6 shape[2,3]", lambda: memoryview(bytes(6)).cast("B", shape=[2, 3]).tolist())

print("== casting an empty view without a shape still works ==")
show("empty cast B", lambda: memoryview(b"").cast("B").tolist())
show("empty cast I", lambda: memoryview(b"").cast("I").tolist())
