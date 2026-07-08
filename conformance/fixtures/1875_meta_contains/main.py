# item in cls runs the metaclass __contains__ with the class as self, the way
# an enum decides membership by member. The result is taken through truthiness.
class Meta(type):
    def __contains__(cls, x):
        return x in cls._members

class Roster(metaclass=Meta):
    _members = ["a", "b", "c"]

print("a" in Roster)
print("z" in Roster)
print("z" not in Roster)

# A metaclass with only __iter__ answers membership by scanning that iterator.
class IterMeta(type):
    def __iter__(cls):
        return iter(cls._items)

class Bag(metaclass=IterMeta):
    _items = [1, 2, 3]

print(2 in Bag)
print(9 in Bag)

# A plain class is neither container nor iterable.
class Plain:
    pass

try:
    1 in Plain
except TypeError as e:
    print(type(e).__name__)
