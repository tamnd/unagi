# array.array is a subclassable value base: a subclass carries an arrayObject
# payload, so it constructs, reprs under its own type name, answers isinstance
# and issubclass, runs the full sequence protocol and the inherited array
# methods, concatenates and repeats to a plain base array, compares by value
# across arrays and array subclasses, and stays unhashable like its base.
import array
import copy


def show(label, fn):
    try:
        v = fn()
        print(label, "=>", type(v).__name__, repr(v))
    except Exception as e:
        print(label, "=>", type(e).__name__, ":", e)


class A(array.array):
    pass


class U(A):
    pass


class Tagged(array.array):
    def __init__(self, *a):
        super().__init__()
        self.tag = "t"


class TenX(array.array):
    def append(self, x):
        super().append(x * 10)


# construction and repr name the subclass, not "array"
show("A('i', [1, 2, 3])", lambda: A('i', [1, 2, 3]))
show("A('d', [1.5, 2.5])", lambda: A('d', [1.5, 2.5]))
show("A('i') empty", lambda: A('i'))
show("U('i', [4, 5])", lambda: U('i', [4, 5]))

a = A('i', [10, 20, 30])

# type identity, isinstance and issubclass across the layout
show("type(a)", lambda: type(a))
show("isinstance(a, array.array)", lambda: isinstance(a, array.array))
show("isinstance(a, A)", lambda: isinstance(a, A))
show("isinstance(U('i'), A)", lambda: isinstance(U('i'), A))
show("issubclass(A, array.array)", lambda: issubclass(A, array.array))
show("issubclass(U, array.array)", lambda: issubclass(U, array.array))

# sequence protocol
show("len(a)", lambda: len(a))
show("list(a)", lambda: list(a))
show("a[1]", lambda: a[1])
show("a[-1]", lambda: a[-1])
show("a[1:]", lambda: a[1:])
show("a[::2]", lambda: a[::2])
show("20 in a", lambda: 20 in a)
show("99 in a", lambda: 99 in a)

# data attributes and inherited methods
show("a.typecode", lambda: a.typecode)
show("a.itemsize", lambda: a.itemsize)
show("a.tolist()", lambda: a.tolist())
show("a.count(20)", lambda: a.count(20))
show("a.index(30)", lambda: a.index(30))
show("a.tobytes()", lambda: a.tobytes())


def mutate(build, op):
    x = build()
    op(x)
    return x


show("append", lambda: mutate(lambda: A('i', [1, 2, 3]), lambda x: x.append(4)))
show("setitem", lambda: mutate(lambda: A('i', [1, 2, 3]), lambda x: x.__setitem__(0, 99)))
show("delitem", lambda: mutate(lambda: A('i', [1, 2, 3]), lambda x: x.__delitem__(1)))
show("extend", lambda: mutate(lambda: A('i', [1, 2, 3]), lambda x: x.extend([4, 5])))
show("insert", lambda: mutate(lambda: A('i', [1, 2, 3]), lambda x: x.insert(1, 99)))
show("reverse", lambda: mutate(lambda: A('i', [1, 2, 3]), lambda x: x.reverse()))
show("pop", lambda: mutate(lambda: A('i', [1, 2, 3]), lambda x: x.pop()))
show("slice-set", lambda: mutate(lambda: A('i', [1, 2, 3]), lambda x: x.__setitem__(slice(0, 2), array.array('i', [7, 8, 9]))))

# operators return a plain base array
show("A + array", lambda: A('i', [1, 2]) + array.array('i', [3, 4]))
show("A + A", lambda: A('i', [1, 2]) + A('i', [3, 4]))
show("A + list", lambda: A('i', [1, 2]) + [3, 4])
show("A * 2", lambda: A('i', [1, 2]) * 2)
show("3 * A", lambda: 3 * A('i', [1, 2]))
show("iadd", lambda: mutate(lambda: A('i', [1, 2]), lambda x: x.__iadd__(array.array('i', [3, 4]))))
show("imul", lambda: mutate(lambda: A('i', [1, 2]), lambda x: x.__imul__(3)))

# comparison by value across arrays and subclasses
show("A == array", lambda: A('i', [1, 2]) == array.array('i', [1, 2]))
show("A == A", lambda: A('i', [1, 2]) == A('i', [1, 2]))
show("A == list", lambda: A('i', [1, 2]) == [1, 2])
show("A < A", lambda: A('i', [1, 2]) < A('i', [1, 3]))
show("array == A", lambda: array.array('i', [1, 2]) == A('i', [1, 2]))

# bool follows the payload length
show("bool(A('i'))", lambda: bool(A('i')))
show("bool(A('i', [1]))", lambda: bool(A('i', [1])))

# a custom __init__ subclass keeps its own attributes
show("Tagged", lambda: Tagged('i', [1, 2, 3]))
show("Tagged.tag", lambda: Tagged('i', [1, 2, 3]).tag)

# super() reaches the base method
show("super().append", lambda: mutate(lambda: TenX('i', [1]), lambda x: x.append(5)))

# copies preserve the subclass type and contents
show("copy.copy", lambda: copy.copy(A('i', [1, 2])))
show("copy.deepcopy", lambda: copy.deepcopy(A('i', [1, 2])))

# array.array stays unhashable through the subclass
show("hash(A('i', [1]))", lambda: hash(A('i', [1])))

# a subclass array works as an initializer for a plain array
show("array.array('i', A(...))", lambda: array.array('i', A('i', [5, 6, 7])))
