log = []


class Meta(type):
    @property
    def label(cls):
        return cls._label

    @label.setter
    def label(cls, value):
        log.append("set " + value)
        cls._label = value

    @label.deleter
    def label(cls):
        log.append("del")
        del cls._label

    slot = property(lambda cls: cls._slot, None)  # no setter


class C(metaclass=Meta):
    _label = "init"


print(C.label)
C.label = "changed"
print(C.label)
print(log)
del C.label
print(log)
try:
    C.slot = 1
except AttributeError as e:
    print("err:", e)

# A plain (non-descriptor) metaclass attr does not intercept: the write lands in
# the class dict and shadows nothing on the metaclass.
Meta.kind = "meta-kind"
C.kind = "class-kind"
print(C.kind)
