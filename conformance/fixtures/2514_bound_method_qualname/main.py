import collections


def p(label, v):
    print(label, "=>", repr(v))


# A bound builtin method names __qualname__ after the type it was read off, the
# way CPython names a bound method after the receiver's own type. bool reads
# bool.name even for a method it inherits from int, and a dict subclass reads its
# own class name. __name__ stays the bare method name.
print("== an instance method names its receiver's type ==")
p("[1, 2].append.__qualname__", [1, 2].append.__qualname__)
p("[1, 2].append.__name__", [1, 2].append.__name__)
p("'ab'.upper.__qualname__", "ab".upper.__qualname__)
p("(255).to_bytes.__qualname__", (255).to_bytes.__qualname__)
p("(255).bit_length.__qualname__", (255).bit_length.__qualname__)
p("True.bit_length.__qualname__", True.bit_length.__qualname__)
p("True.conjugate.__qualname__", True.conjugate.__qualname__)
p("(1.5).is_integer.__qualname__", (1.5).is_integer.__qualname__)
p("(1.5).hex.__qualname__", (1.5).hex.__qualname__)
p("(3 + 4j).conjugate.__qualname__", (3 + 4j).conjugate.__qualname__)
p("b'x'.hex.__qualname__", b"x".hex.__qualname__)
p("bytearray(b'x').hex.__qualname__", bytearray(b"x").hex.__qualname__)
p("{'a': 1}.get.__qualname__", {"a": 1}.get.__qualname__)
p("{1, 2}.add.__qualname__", {1, 2}.add.__qualname__)
p("frozenset().union.__qualname__", frozenset().union.__qualname__)
p("(1, 2, 1).count.__qualname__", (1, 2, 1).count.__qualname__)
p("memoryview(b'x').hex.__qualname__", memoryview(b"x").hex.__qualname__)
p("OrderedDict().move_to_end.__qualname__", collections.OrderedDict().move_to_end.__qualname__)
p("defaultdict(int).get.__qualname__", collections.defaultdict(int).get.__qualname__)

print("== a numeric operator dunder names its receiver's type ==")
p("(5).__add__.__qualname__", (5).__add__.__qualname__)
p("(5).__neg__.__qualname__", (5).__neg__.__qualname__)
p("(5).__divmod__.__qualname__", (5).__divmod__.__qualname__)
p("(5).__hash__.__qualname__", (5).__hash__.__qualname__)
p("True.__and__.__qualname__", True.__and__.__qualname__)
p("(1.5).__add__.__qualname__", (1.5).__add__.__qualname__)
p("(3 + 4j).__add__.__qualname__", (3 + 4j).__add__.__qualname__)

print("== __name__ stays bare and the method still calls ==")
p("(255).to_bytes.__name__", (255).to_bytes.__name__)
p("(5).__add__.__name__", (5).__add__.__name__)
p("[1].append(2) result", (lambda x: (x.append(2), x)[1])([1]))
p("'ab'.upper()", "ab".upper())
p("(5).__add__(3)", (5).__add__(3))
