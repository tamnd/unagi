# __getattr__ is the attribute-miss fallback: normal resolution runs first, and
# only a miss calls __getattr__(name). An existing attribute never reaches it,
# and an AttributeError raised inside it propagates unchanged.
class Proxy:
    def __init__(self):
        self.real = 1

    def __getattr__(self, name):
        return "dynamic:" + name


p = Proxy()
print(p.real)
print(p.missing)
print(p.other)


class Strict:
    def __getattr__(self, name):
        raise AttributeError("no attribute " + name)


s = Strict()
try:
    print(s.x)
except AttributeError as e:
    print(e)


class Shadow:
    z = 5

    def __getattr__(self, name):
        return "hidden"


print(Shadow().z)
