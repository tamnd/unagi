# dict(mapping) must build from the keys() protocol, not by iterating the
# object as a sequence of (key, value) pairs. A plain object exposing keys() and
# __getitem__ is a mapping (CPython's PyMapping_Keys probe).

class M:
    def keys(self):
        return ["a", "b", "c"]
    def __getitem__(self, k):
        return {"a": 1, "b": 2, "c": 3}[k]

print(dict(M()))

# keys() may return any iterable, and the order it yields is preserved.
class Ordered:
    def keys(self):
        return iter(("z", "y", "x"))
    def __getitem__(self, k):
        return k.upper()

print(dict(Ordered()))

# The sequence-of-pairs path still works when there is no keys().
print(dict([("p", 1), ("q", 2)]))
print(dict((("r", 3),)))

# A real dict and a dict subclass still copy by their keys.
print(dict({"m": 1, "n": 2}))

from collections import OrderedDict
print(dict(OrderedDict([("u", 1), ("v", 2)])))

# Keyword arguments combine with a mapping positional.
print(dict(M(), d=4, a=9))

# The wider consumer: xml.sax's ContentHandler does dict(AttributesImpl(attrs)).
import xml.sax
from xml.sax.handler import ContentHandler

class H(ContentHandler):
    def __init__(self):
        self.rows = []
    def startElement(self, name, attrs):
        self.rows.append((name, dict(attrs)))

h = H()
xml.sax.parseString(b'<doc><item id="7" k="v"/><plain/></doc>', h)
for row in h.rows:
    print(row)
