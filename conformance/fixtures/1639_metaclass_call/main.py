# A user metaclass __call__ intercepts C(...), running instead of the default
# creation protocol; super().__call__ reaches the ordinary __new__/__init__ path.

class Tagging(type):
    def __call__(cls, *args, **kw):
        print("Tagging.__call__", cls.__name__, args, kw)
        obj = super().__call__(*args, **kw)
        obj.tagged = True
        return obj


class Point(metaclass=Tagging):
    def __init__(self, x, y):
        self.x = x
        self.y = y


p = Point(1, 2)
print("x, y, tagged:", p.x, p.y, p.tagged)

# Keywords thread through super().__call__.
q = Point(x=3, y=4)
print("kw x, y, tagged:", q.x, q.y, q.tagged)


# A metaclass __call__ can cache and fully replace creation.
class Cached(type):
    def __init__(cls, name, bases, ns):
        super().__init__(name, bases, ns)
        cls._cache = {}

    def __call__(cls, key):
        if key in cls._cache:
            return cls._cache[key]
        obj = super().__call__(key)
        cls._cache[key] = obj
        return obj


class Node(metaclass=Cached):
    def __init__(self, key):
        self.key = key


a = Node("x")
b = Node("x")
c = Node("y")
print("cached a is b:", a is b)
print("distinct a is c:", a is c)
print("a.key, c.key:", a.key, c.key)


# A metaclass __call__ may return a foreign object outright.
class Constant(type):
    def __call__(cls, *args, **kw):
        return "always this"


class Whatever(metaclass=Constant):
    def __init__(self):
        raise RuntimeError("never runs")


print("constant:", Whatever())


# A class on the default type metatype creates its instance directly.
class Plain:
    def __init__(self, v):
        self.v = v


print("plain:", Plain(9).v)
