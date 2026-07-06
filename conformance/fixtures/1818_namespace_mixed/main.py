# A namespace package can hold a regular package: plugins has no __init__.py so
# it is a namespace, while plugins.core has one and executes as an ordinary
# package. A submodule that does not exist under the namespace still raises
# ModuleNotFoundError.
import plugins.core

print("plugins file:", plugins.__file__)
print("core is regular:", plugins.core.__file__ is not None)

import plugins.core.impl

print("impl:", plugins.core.impl.tag)

try:
    import plugins.missing
except ModuleNotFoundError as e:
    print("missing:", e)
