# A class object dispatches iteration to its metaclass __iter__, so a for-loop,
# list(), and tuple unpacking walk what the metaclass yields rather than the
# class instances. This is the shape enum runs when it unpacks a member-carrying
# class, STRICT, CONFORM, EJECT, KEEP = FlagBoundary.


class Meta(type):
    def __iter__(cls):
        # Yield the class's registered items in definition order.
        return (item for item in cls._items)


class Colors(metaclass=Meta):
    _items = ["red", "green", "blue"]


# list() and a for-loop consume the metaclass iterator.
print(list(Colors))
for c in Colors:
    print(c)

# Tuple unpacking iterates the class, the enum unpack shape.
a, b, c = Colors
print(a, b, c)


# A four-item class unpacks into four names, none skipped.
class Four(metaclass=Meta):
    _items = [1, 2, 3, 4]


w, x, y, z = Four
print(w, x, y, z)


# A plain class with no metaclass __iter__ stays non-iterable.
class Plain:
    pass


try:
    iter(Plain)
except TypeError as e:
    print("TypeError", "not iterable" in str(e))
