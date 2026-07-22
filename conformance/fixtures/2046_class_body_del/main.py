# A class body runs del as DELETE_NAME against the class namespace, so a name
# bound earlier in the body drops before the class is built and a delete of an
# unbound name is a NameError. Stdlib modules like textwrap del their scratch
# names at class scope, so the class-body lowering has to route del through the
# namespace builder rather than the function-local path.
class C:
    keep = 1
    scratch = 2
    del scratch
    tail = 3

print(C.keep, C.tail)
print(hasattr(C, "scratch"))

# A del inside a loop in the body clears the temporary after the loop.
class D:
    total = 0
    for i in (10, 20, 30):
        total += i
    del i

print(D.total)
print(hasattr(D, "i"))

# Deleting a name the body never bound raises NameError at the del site.
try:
    class E:
        del missing
except NameError as e:
    print("NameError:", e)
