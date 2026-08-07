import array


def show(label, e):
    try:
        print(label, repr(e()))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, str(ex))


class Idx:
    def __index__(self):
        return 2


class BadIdx:
    def __index__(self):
        return "x"


# array subscripts and the index-taking methods run the index through __index__
# the way CPython feeds a subscript to PyNumber_Index, so an object spelling
# __index__ indexes as its value and a bool counts as 0 or 1.
def sub_set():
    a = array.array("i", [10, 20, 30, 40])
    a[Idx()] = 99
    return a.tolist()


def sub_del():
    a = array.array("i", [10, 20, 30, 40])
    del a[Idx()]
    return a.tolist()


def method_insert():
    a = array.array("i", [10, 20, 30])
    a.insert(Idx(), 99)
    return a.tolist()


show("get-idx", lambda: array.array("i", [10, 20, 30, 40])[Idx()])
show("get-bool", lambda: array.array("i", [10, 20, 30, 40])[True])
show("get-neg", lambda: array.array("i", [10, 20, 30, 40])[-1])
show("set-idx", sub_set)
show("del-idx", sub_del)
show("insert-idx", method_insert)
show("pop-idx", lambda: array.array("i", [10, 20, 30, 40]).pop(Idx()))
show("pop-default", lambda: array.array("i", [10, 20, 30]).pop())

# A bad __index__ return propagates its non-int TypeError from every one of these
# entry points, and a float or str index (no __index__) keeps the type error each
# form spells: the subscript names array indices, the methods name the argument.
show("get-bad", lambda: array.array("i", [1, 2, 3])[BadIdx()])
show("insert-bad", lambda: array.array("i", [1, 2]).insert(BadIdx(), 5))
show("pop-bad", lambda: array.array("i", [1, 2]).pop(BadIdx()))
show("get-float", lambda: array.array("i", [1, 2, 3])[1.0])
show("get-str", lambda: array.array("i", [1, 2, 3])["x"])
show("insert-float", lambda: array.array("i", [1, 2]).insert(1.0, 5))
show("pop-str", lambda: array.array("i", [1, 2]).pop("x"))

# The range checks still hold on a coerced index, so an out-of-range __index__ is
# the ordinary index error each operation gives.
show("get-oob", lambda: array.array("i", [1, 2])[Idx()])
show("pop-oob", lambda: array.array("i", [1, 2]).pop(Idx()))
