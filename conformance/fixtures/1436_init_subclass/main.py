# type.__new__ calls the nearest base's __init_subclass__ on each new subclass,
# after __set_name__. It is an implicit classmethod, so the new class arrives as
# cls and the class keyword arguments follow. The base's own creation does not
# fire it, and a keyword argument with no hook to receive it is a TypeError.
class Base:
    registry = []

    def __init_subclass__(cls, kind="plain", **rest):
        Base.registry.append((cls.__name__, kind, sorted(rest.items())))


class Alpha(Base, kind="a"):
    pass


class Beta(Base, kind="b", extra=1):
    pass


class Gamma(Base):
    pass


for row in Base.registry:
    print(row)

try:

    class Orphan(kind="x"):
        pass

except TypeError as e:
    print(e)
