# len(cls) runs the metaclass __len__ with the class as self, the way an enum
# reports its member count. A metaclass without __len__ leaves the class not
# sized, and an instance keeps its own __len__ path.
class Meta(type):
    def __len__(cls):
        return len(cls._members)

class Roster(metaclass=Meta):
    _members = ["a", "b", "c"]

print(len(Roster))

class Empty(metaclass=Meta):
    _members = []

print(len(Empty))

# A plain class still has no len.
class Plain:
    pass

try:
    len(Plain)
except TypeError as e:
    print(type(e).__name__)

# An instance __len__ is unaffected by the metaclass path.
class Sized:
    def __len__(self):
        return 7

print(len(Sized()))
