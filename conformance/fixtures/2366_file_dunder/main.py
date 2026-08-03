import os

# __file__ names the module's source file. The exact path depends on where the
# program was compiled, so these checks are structural: they confirm the value is
# a string that names this file, not its literal path.
print("is str:", isinstance(__file__, str))
print("basename:", os.path.basename(__file__))
print("in globals:", "__file__" in globals())


def read_file():
    # A read inside a function sees the same module-level value.
    return __file__


print("same inside func:", read_file() == __file__)

# dirname(__file__) is the directory the source sits in, so joining a sibling name
# onto it and checking the leaf keeps the check path-independent.
sibling = os.path.join(os.path.dirname(__file__), "data.txt")
print("sibling leaf:", os.path.basename(sibling))
