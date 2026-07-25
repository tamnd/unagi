# type.__new__(metacls, name, bases, ns) is the explicit class-construction form.
# typing.py builds NamedTuple with type.__new__(NamedTupleMeta, 'NamedTuple', (),
# {}), so type.__new__ has to resolve and accept a user metaclass, and the `type`
# builtin itself, as the metaclass argument.


class Meta(type):
    pass


X = type.__new__(Meta, "X", (), {"a": 1})
print(type(X).__name__, X.__name__, X.a, [b.__name__ for b in X.__bases__])

Y = type.__new__(type, "Y", (object,), {})
print(type(Y).__name__, Y.__name__)

# A base carried through builds the MRO the ordinary way.
Z = type.__new__(type, "Z", (X,), {"b": 2})
print(Z.__name__, Z.a, Z.b, [c.__name__ for c in Z.__mro__])
