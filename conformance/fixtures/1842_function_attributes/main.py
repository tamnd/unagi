# A Python function carries writable attributes: the __dict__ of arbitrary names
# it grows on assignment, plus the __name__, __qualname__, __doc__, __module__,
# and __annotations__ slots. A fresh function starts with an empty __dict__ and
# an empty __annotations__, __module__ __main__, and __doc__ None until code
# assigns to them. The slots reject the wrong type the way CPython does.


def outer():
    pass


class C:
    def method(self):
        pass


print(outer.__name__, outer.__qualname__)
print(C.method.__name__, C.method.__qualname__)
print(outer.__module__)
print(outer.__doc__)
print(outer.__dict__)
print(outer.__annotations__)

# Arbitrary attributes land in __dict__ and read back.
outer.tag = 7
outer.owner = "team"
print(outer.tag, outer.owner)
print(sorted(outer.__dict__.items()))

# The __dict__ identity is stable, so a write through it reaches the function.
d = outer.__dict__
d["late"] = 99
print(outer.late)

# Deleting an attribute removes it; deleting a missing one is AttributeError.
del outer.tag
print(sorted(outer.__dict__.keys()))


def show(fn):
    try:
        return fn()
    except Exception as e:
        return type(e).__name__ + ": " + str(e)


print(show(lambda: delattr(outer, "tag")))

# The name slots are writable and enforce a string.
outer.__name__ = "renamed"
outer.__qualname__ = "mod.renamed"
print(outer.__name__, outer.__qualname__)
print(show(lambda: setattr(outer, "__name__", 5)))
print(show(lambda: setattr(outer, "__qualname__", 5)))

# __doc__ takes any value and reverts to None on delete.
outer.__doc__ = "a summary"
print(outer.__doc__)
del outer.__doc__
print(outer.__doc__)

# __annotations__ and __dict__ must be set to the right container types.
outer.__annotations__ = {"x": int}
print(outer.__annotations__)
print(show(lambda: setattr(outer, "__annotations__", 5)))
print(show(lambda: setattr(outer, "__dict__", 5)))
