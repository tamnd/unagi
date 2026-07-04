# type.__new__ calls __set_name__(owner, name) on every class-body value whose
# type defines it, in definition order, right after the class exists. A
# descriptor uses it to learn the attribute name it was bound to. Inherited
# names do not fire it again, and a value whose type has no __set_name__ is
# skipped.
class Field:
    def __init__(self, label):
        self.label = label
        self.name = None

    def __set_name__(self, owner, name):
        print("set_name", owner.__name__, name, self.label)
        self.name = name


class Plain:
    pass


class Model:
    first = Field("a")
    second = Field("b")
    tag = Plain()


print(Model.first.name)
print(Model.second.name)


class Sub(Model):
    third = Field("c")


print(Sub.first.name)
print(Sub.third.name)
