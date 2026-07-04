class Counter:
    def __init__(self, start=0, step=1):
        self.value = start
        self.step = step

    def bump(self, by=None):
        self.value += self.step if by is None else by
        return self.value

    def label(self, prefix="count", sep=": "):
        return prefix + sep + str(self.value)


c = Counter()
print(c.value, c.step)
print(c.bump())
print(c.bump(10))
print(c.label())
print(c.label("total"))

d = Counter(5, 2)
print(d.bump())
print(d.label("sum", " = "))
