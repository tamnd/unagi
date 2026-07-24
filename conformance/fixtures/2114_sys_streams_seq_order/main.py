import sys

sys.stdout.write("hello ")
sys.stdout.write("world\n")
sys.stdout.writelines(["a", "b", "c\n"])
print(sys.stdout.name, sys.stdout.mode, sys.stdout.encoding, sys.stdout.errors)
print(sys.stdout.writable(), sys.stdout.readable(), sys.stdout.isatty())
print(sys.stdout.fileno(), sys.stderr.fileno(), sys.stdin.fileno())
print(sys.stdin.readable(), sys.stdin.writable())
print(sys.stdout is sys.__stdout__, sys.stderr is sys.__stderr__)
print(sys.stdout.write("count: ") == 7)
print()


class Key:
    def __init__(self, obj):
        self.obj = obj

    def __lt__(self, other):
        return self.obj < other.obj

    def __eq__(self, other):
        return self.obj == other.obj

    def __repr__(self):
        return "Key(%r)" % (self.obj,)


# Sorting a list of tuples whose elements are user instances now dispatches the
# element __eq__ and __lt__ the way CPython's sequence comparison does.
pairs = [(Key(2), Key("z")), (Key(1), Key("y")), (Key(2), Key("a")), (Key(1), Key("x"))]
print(sorted(pairs))

# Plain tuple ordering with an instance element that ties on the first field.
print((Key(1), Key(3)) < (Key(1), Key(5)))
print([Key(1), Key(2)] < [Key(1), Key(2), Key(0)])
print(sorted([[Key(3)], [Key(1)], [Key(2)]]))
