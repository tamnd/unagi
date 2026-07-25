# dir() of a module is the sorted, de-duplicated list of its namespace names,
# its __dict__ keys with the dunders included. The exact set a native module
# exposes is an implementation detail, so this checks the shape dir() promises
# rather than the contents: a sorted list of unique strings that carries the
# identity dunders and the module's own public names.
import os
import json

names = dir(os)

# dir() returns a list.
print(type(names) is list)

# It is sorted and de-duplicated.
print(names == sorted(names))
print(len(names) == len(set(names)))

# Every element is a string.
print(all(isinstance(n, str) for n in names))

# The identity dunders a module always carries are present.
print("__name__" in names)
print("__doc__" in names)

# A public name the module defines shows up.
print("getcwd" in names)

# dir() reflects a name bound onto the module after import.
os.sentinel_marker = 7
print("sentinel_marker" in dir(os))

# It works the same for another imported module.
jn = dir(json)
print(jn == sorted(jn), "dumps" in jn, "loads" in jn)
