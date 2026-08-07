import array


def show(label, fn):
    try:
        print(label, repr(fn()))
    except Exception as ex:
        print(label, "ERR", type(ex).__name__, str(ex))


class Idx:
    def __index__(self):
        return 2


# array exposes its arithmetic slots as named methods the way CPython does, so
# __add__ concatenates two arrays into a fresh one, __mul__ and __rmul__ repeat
# the elements with the count read through __index__, and __iadd__ and __imul__
# mutate in place and hand back the same object.
show("has-add", lambda: hasattr(array.array('i', [1]), '__add__'))
show("has-mul", lambda: hasattr(array.array('i', [1]), '__mul__'))
show("has-rmul", lambda: hasattr(array.array('i', [1]), '__rmul__'))
show("has-iadd", lambda: hasattr(array.array('i', [1]), '__iadd__'))
show("has-imul", lambda: hasattr(array.array('i', [1]), '__imul__'))

show("add", lambda: array.array('i', [1, 2]).__add__(array.array('i', [3])))
show("add-wrongtype", lambda: array.array('i', [1]).__add__(5))
show("add-list", lambda: array.array('i', [1]).__add__([3, 4]))
show("add-diffcode", lambda: array.array('i', [1]).__add__(array.array('f', [1.0])))
show("add-arity0", lambda: array.array('i', [1]).__add__())
show("add-arity2", lambda: array.array('i', [1]).__add__(array.array('i', [1]), array.array('i', [1])))

show("mul", lambda: array.array('i', [1, 2]).__mul__(3))
show("mul-idx", lambda: array.array('i', [1, 2]).__mul__(Idx()))
show("mul-bool", lambda: array.array('i', [1, 2]).__mul__(True))
show("mul-neg", lambda: array.array('i', [1, 2]).__mul__(-1))
show("mul-float", lambda: array.array('i', [1, 2]).__mul__(1.5))
show("mul-str", lambda: array.array('i', [1, 2]).__mul__("x"))
show("rmul", lambda: array.array('i', [1, 2]).__rmul__(2))
show("rmul-idx", lambda: array.array('i', [1, 2]).__rmul__(Idx()))
show("mul-arity0", lambda: array.array('i', [1]).__mul__())

show("iadd", lambda: array.array('i', [1]).__iadd__(array.array('i', [2, 3])))
show("iadd-list", lambda: array.array('i', [1]).__iadd__([2, 3]))
show("iadd-wrong", lambda: array.array('i', [1]).__iadd__(5))
show("iadd-diffcode", lambda: array.array('i', [1]).__iadd__(array.array('f', [1.0])))

show("imul", lambda: array.array('i', [1, 2]).__imul__(2))
show("imul-idx", lambda: array.array('i', [1, 2]).__imul__(Idx()))
show("imul-zero", lambda: array.array('i', [1, 2]).__imul__(0))
show("imul-neg", lambda: array.array('i', [1, 2]).__imul__(-1))
show("imul-float", lambda: array.array('i', [1, 2]).__imul__(1.5))


def add_new():
    a = array.array('i', [1])
    return a.__add__(array.array('i', [2])) is not a


def iadd_identity():
    a = array.array('i', [1])
    return a.__iadd__(array.array('i', [9])) is a


def imul_identity():
    a = array.array('i', [1, 2])
    return a.__imul__(2) is a


show("add-new", add_new)
show("iadd-identity", iadd_identity)
show("imul-identity", imul_identity)
