class C:
    def __init__(self):
        self.x = 1


c = C()

# getattr reads an existing attribute and falls back to the default only on a
# missing one.
print(getattr(c, "x"))
print(getattr(c, "y", "default"))

# hasattr answers True/False off the same read.
print(hasattr(c, "x"), hasattr(c, "y"))

# setattr binds a new attribute that getattr then sees.
setattr(c, "z", 42)
print(c.z, getattr(c, "z"))

# delattr removes it again; hasattr and a defaulted getattr confirm the miss.
delattr(c, "x")
print(hasattr(c, "x"), getattr(c, "x", "gone"))

# A missing attribute with no default raises, catchable as usual.
try:
    getattr(c, "nope")
except AttributeError as e:
    print("attr error:", e)

# A non-string name is a TypeError across all four.
for name, call in [
    ("getattr", lambda: getattr(c, 5)),
    ("setattr", lambda: setattr(c, 5, 1)),
    ("hasattr", lambda: hasattr(c, 5)),
    ("delattr", lambda: delattr(c, 5)),
]:
    try:
        call()
    except TypeError as e:
        print(name, "type error:", e)

# Passed around as values they still work.
g = getattr
print(g(c, "z"))
