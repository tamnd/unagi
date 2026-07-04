# A class binds methods and class variables; instances carry their own state.
class Counter:
    kind = "counter"

    def __init__(self, start):
        self.n = start

    def bump(self, d):
        self.n += d
        return self.n

    def value(self):
        return self.n


c = Counter(10)
print(c.bump(5))
print(c.bump(2))
print(c.value())

# A class variable is shared; an instance reads it through the class.
print(Counter.kind, c.kind)

# Attributes can be set after construction, on the instance and on the class.
c.label = "primary"
print(c.label)
Counter.kind = "tally"
print(Counter.kind, c.kind)

# Keyword arguments bind __init__ parameters by name.
class Point:
    def __init__(self, x, y):
        self.x = x
        self.y = y

    def norm2(self):
        return self.x * self.x + self.y * self.y


p = Point(y=4, x=3)
print(p.norm2())

# A method read off an instance is a bound method that keeps its self.
m = p.norm2
print(m())

# A method read off the class takes its self explicitly.
print(Point.norm2(p))

# Two instances of the same class hold independent state.
a = Counter(0)
b = Counter(100)
a.bump(1)
b.bump(1)
print(a.value(), b.value())

# repr of a class spells its qualified name.
print(repr(Counter))
print(str(Point))
