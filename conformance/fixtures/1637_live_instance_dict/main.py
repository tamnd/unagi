class C:
    pass

c = C()
c.a = 1

d = c.__dict__
print("d:", d)
print("c.__dict__ is c.__dict__:", c.__dict__ is c.__dict__)
print("d is c.__dict__:", d is c.__dict__)

d["b"] = 2
print("read c.b after dict write:", c.b)

c.x = 10
print("d sees attr set on instance:", d["x"])

del c.a
print("d after del c.a:", d)
print("vars(c) is c.__dict__:", vars(c) is c.__dict__)

d.clear()
print("vars(c) after clear:", vars(c))
print("still has attr x:", hasattr(c, "x"))


class Desc:
    def __set__(self, obj, value):
        obj.__dict__["_v"] = value
    def __get__(self, obj, owner):
        return obj.__dict__.get("_v", None)


class D:
    v = Desc()


o = D()
o.v = 42
print("descriptor wrote through __dict__:", o.__dict__)
print("read back through descriptor:", o.v)
