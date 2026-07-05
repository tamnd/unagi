class Meta(type):
    kind = "meta-kind"

    def greet(cls):
        return "hi " + cls.__name__

    @property
    def label(cls):
        return "label-of-" + cls.__name__

    @classmethod
    def factory(mcs):
        return "factory-" + mcs.__name__

    @staticmethod
    def helper():
        return "static-helper"

    val = property(lambda cls: "from-meta")


class C(metaclass=Meta):
    tag = "class-tag"

    def m(self):
        return "method-m"


class D(metaclass=Meta):
    val = "from-class-dict"


class E(metaclass=Meta):
    kind = "class-kind"


class Sub(C):
    pass


print(C.greet())
print(C.kind)
print(C.label)
print(C.factory())
print(C.helper())
print(C.tag)
print(D.val)
print(E.kind)
print(Sub.greet())
print(C().m())
