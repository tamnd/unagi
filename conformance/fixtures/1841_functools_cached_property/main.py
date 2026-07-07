# functools.cached_property, the non-data descriptor that computes its value the
# first time an attribute is read off an instance and caches the result in the
# instance dict, so the wrapped function runs once per instance and later reads
# hit the dict directly even after the inputs change. An instance with no
# __dict__ cannot cache and raises TypeError.
import functools
from functools import cached_property


class Circle:
    def __init__(self, radius):
        self.radius = radius
        self.computed = 0

    @cached_property
    def area(self):
        self.computed += 1
        return self.radius * self.radius * 3


a = Circle(2)
print(a.area)
print(a.area)
print(a.computed)

# The cached value sticks even after the input changes, since the function does
# not run again.
a.radius = 100
print(a.area)

# Each instance caches independently.
b = Circle(3)
print(b.area)
print(a.computed, b.computed)

# The descriptor read off the class exposes the wrapped function.
print(Circle.area.func.__name__)


class Slotted:
    __slots__ = ("radius",)

    @cached_property
    def area(self):
        return self.radius * 2


def show(fn):
    try:
        return fn()
    except Exception as e:
        return type(e).__name__ + " " + str(e)


s = Slotted()
s.radius = 5
print(show(lambda: s.area))
