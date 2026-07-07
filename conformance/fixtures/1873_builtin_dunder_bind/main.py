# A class can reassign a builtin string dunder to another slot, the way enum
# sets __str__ = int.__repr__ so a member renders as its bare int. Read off an
# instance the wrapper binds that instance as self, so the call carries the
# value; read off the class it stays unbound.
class M(int):
    __str__ = int.__repr__

m = M(6)
print(str(m))
print(m.__str__())
print(f"{m}")
print(str(M(255)))

# The class-level read hands back the wrapper unbound, so an explicit self works.
print(M.__str__(M(9)))

# A plain int keeps its own str, unaffected by the subclass override.
print(str(6), repr(6))
