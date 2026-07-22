# Subclassing a collections.namedtuple result, the shape tokenize.TokenInfo
# takes: a class whose only base is namedtuple(...), adding a property and a
# __repr__. The subclass instances stay tuples carrying the fields, keep tuple
# behavior, and answer the namedtuple helpers while the subclass methods win.
import collections
from collections import namedtuple


class TokenInfo(namedtuple("TokenInfo", ["type", "string", "start", "end", "line"])):
    @property
    def upper_string(self):
        return self.string.upper()

    def summary(self):
        return f"{self.type}:{self.string}"


t = TokenInfo(1, "name", (1, 0), (1, 4), "name\n")

# Field access by name and by index, tuple length and slicing.
print(t.type, t.string, t.start, t.end, t.line)
print(t[0], t[1], t[-1], len(t), t[1:3])

# Keyword construction binds the fields the same way.
print(TokenInfo(type=2, string="x", start=(0, 0), end=(0, 1), line="x"))

# The subclass methods and property win over the tuple payload.
print(t.upper_string, t.summary())

# Tuple behavior: equality with a bare tuple, membership, unpacking, iteration.
print(t == (1, "name", (1, 0), (1, 4), "name\n"))
print("name" in t)
typ, s, st, en, ln = t
print(typ, s, ln)
print([type(x).__name__ for x in t])

# isinstance sees the subclass and tuple; type identity is the subclass.
print(isinstance(t, TokenInfo), isinstance(t, tuple), type(t).__name__)

# repr with no override reads the namedtuple layout.
print(repr(t))

# The namedtuple helpers resolve on the instance and the class, and _make and
# _replace keep the subclass type.
print(t._fields)
print(t._asdict())
r = t._replace(string="other")
print(r, type(r).__name__, r.upper_string)
m = TokenInfo._make([9, "z", (2, 0), (2, 1), "z\n"])
print(m, type(m).__name__)
print(TokenInfo._fields)
print(t.count("name"), t.index("name"))


# A subclass with a default field.
class Row(namedtuple("Row", "a b c", defaults=[0])):
    pass


print(Row(1, 2))
print(Row._field_defaults)


# A subclass of the subclass keeps the field metadata.
class LabeledToken(TokenInfo):
    pass


lt = LabeledToken(5, "kw", (3, 0), (3, 2), "kw\n")
print(lt.string, lt.summary(), type(lt).__name__, isinstance(lt, TokenInfo))
