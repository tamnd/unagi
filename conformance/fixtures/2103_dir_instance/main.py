# dir(obj) lists an object's attribute names. For a user instance it is the
# sorted union of the instance's own attributes, every name across its type's
# MRO, and the object base set. configparser leans on this: SectionProxy scans
# dir(self._parser) for the getint/getfloat converters.
class Base:
    x = 1

    def foo(self):
        return 1


class Der(Base):
    def bar(self):
        return 2


d = Der()
d.inst = 9
names = dir(d)

# The instance attribute, the class variable, and both methods all show up.
print("inst" in names, "x" in names, "foo" in names, "bar" in names)
# The object base dunders are present, __qualname__ is not.
print("__class__" in names, "__init__" in names, "__dict__" in names)
print("__qualname__" in names)
# The list is sorted and free of duplicates.
print(names == sorted(names))
print(len(names) == len(set(names)))
# The public names come out exactly, the way configparser filters them.
print([n for n in names if not n.startswith("_")])
# A dunder defined on the class is not doubled.
print(dir(d).count("bar"), dir(d).count("__init__"))


# A class that defines __dir__ decides the whole list, sorted.
class Custom:
    def __dir__(self):
        return ["zebra", "apple", "apple", "mango"]


print(dir(Custom()))


# The converter-scan pattern configparser runs.
class Parser:
    def getint(self):
        pass

    def getfloat(self):
        pass

    def get(self):
        pass

    def read(self):
        pass


getters = [g for g in dir(Parser()) if g.startswith("get")]
print(sorted(getters))

# Arity and the no-argument form raise catchable TypeErrors.
try:
    dir(1, 2)
except TypeError as e:
    print("arity:", e)
