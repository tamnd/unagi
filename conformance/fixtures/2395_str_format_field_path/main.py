# str.format resolves the '.attr' and '[key]' path of a replacement field,
# walking getattr for a dotted step and getitem for a bracketed one. A bracket
# key of only digits is an integer index; any other key, a leading '-' included,
# is a string key.


class Point:
    def __init__(self, x, y):
        self.x = x
        self.y = y


p = Point(3, 4)
data = {"items": [10, 20, 30], "meta": {"name": "grid"}}

# Attribute paths off a positional and a named field.
print("{0.x},{0.y}".format(p))
print("{pt.x}+{pt.y}".format(pt=p))

# Index paths: integer index into a list, string key into a dict.
print("{0[1]}".format([100, 200, 300]))
print("{d[meta][name]}".format(d=data))
print("{d[items][0]}/{d[items][2]}".format(d=data))

# Chained mix of attribute and index steps.
print("{0[items][1]}".format(data))
print("{a.y}".format(a=p))

# A complex number exposes .real and .imag through the same path.
print("{z.real}:{z.imag}".format(z=3 + 4j))
print("{0.real}".format(2 + 5j))

# Error paths match CPython's wording.
for label, thunk in [
    ("neg key on list", lambda: "{0[-1]}".format([1, 2, 3])),
    ("index oob", lambda: "{0[9]}".format([1])),
    ("missing attr", lambda: "{0.nope}".format(1)),
    ("missing key", lambda: "{d[absent]}".format(d={})),
    ("empty attr", lambda: "{0.}".format(1)),
    ("empty key", lambda: "{0[]}".format([1])),
]:
    try:
        print(label, "->", thunk())
    except Exception as e:
        print(label, "RAISES", type(e).__name__, str(e))
