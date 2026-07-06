# A module body that raises comes out of the registry again, so a retry runs
# the body a second time; the package that imported fine stays cached through
# a failed submodule import. The last import is uncaught for the traceback.
try:
    import fragile
except ValueError as e:
    print("caught:", e)

try:
    import fragile
except ValueError as e:
    print("caught again:", e)

import pkg

print("pkg ok")

try:
    import pkg.bad
except RuntimeError as e:
    print("caught:", e)

import pkg

print("pkg still cached")

try:
    import pkg.bad
except RuntimeError as e:
    print("bad reran:", e)

import fragile

print("unreachable")
