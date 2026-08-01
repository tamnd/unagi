# A dict subclass whose __eq__/__or__ decline a non-subclass operand (the shape
# collections.Counter uses) must let the plain dict's reflected __eq__/__ror__
# settle the comparison and the union, and a bare dict subclass with no override
# compares against a plain dict by contents.


class C(dict):
    def __eq__(self, other):
        if not isinstance(other, C):
            return NotImplemented
        return dict(self) == dict(other)

    __hash__ = None

    def __or__(self, other):
        if not isinstance(other, C):
            return NotImplemented
        return "C-union"


class Plain(dict):
    pass


c = C(a=1)

# __eq__ declines the plain dict, so dict.__eq__ compares by contents both ways.
print("c == dict:", c == {"a": 1})
print("dict == c:", {"a": 1} == c)
print("c != dict:", c != {"a": 2})
print("dict != c:", {"a": 2} != c)

# __or__ declines, so dict.__ror__ merges into a fresh plain dict (right wins).
u = c | {"a": 2, "b": 3}
print("c | dict:", u, type(u).__name__)
# The subclass's own __or__ still runs for a same-type operand.
print("c | C:", c | C(a=9))

# A bare dict subclass with no override compares by contents.
p = Plain(x=1, y=2)
print("plain == dict:", p == {"x": 1, "y": 2})
print("dict == plain:", {"x": 1, "y": 2} == p)
print("plain != dict:", p != {"x": 1})
print("plain == other:", p == {"x": 1, "y": 3})
