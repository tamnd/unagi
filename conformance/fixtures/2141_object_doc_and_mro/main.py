# Every object answers __doc__ rather than raising. email._policybase's
# _extend_docstrings walks a class dict and reads attr.__doc__ on every value it
# holds, scalars and instances alike, so the whole surface has to answer. The
# concrete text a builtin carries is version specific, so these check the stable
# invariant: the read resolves to a string or None instead of AttributeError.
def doc_ok(value):
    d = value.__doc__
    return d is None or isinstance(d, str)


# Builtin scalar values answer __doc__.
print(doc_ok(5))
print(doc_ok(1.5))
print(doc_ok(2 + 3j))
print(doc_ok(True))

# str, bytes and the container builtins answer through the shared tail.
print(doc_ok("x"))
print(doc_ok(b"y"))
print(doc_ok([1]))
print(doc_ok({}))

# A builtin type object answers __doc__ too.
print(doc_ok(int))
print(doc_ok(type(5)))


# A user class instance reads __doc__ off its type: None when the class has no
# docstring, the docstring itself when it does.
class Plain:
    pass


class Documented:
    """A short docstring."""


print(Plain().__doc__)
print(Documented().__doc__)
print(Documented.__doc__)


# type.mro() returns the method resolution order as a fresh list, the mutable
# sibling of the __mro__ tuple.
class Base:
    pass


class Mid(Base):
    pass


class Leaf(Mid):
    pass


order = Leaf.mro()
print(type(order).__name__)
print([c.__name__ for c in order])
print(tuple(order) == Leaf.__mro__)
print(Leaf.mro() is not Leaf.mro())
