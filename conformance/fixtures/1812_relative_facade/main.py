# The facade pattern with relative forms: __init__ pulls in its submodule and
# re-exports a function, and a function-body relative import inside a plain
# submodule resolves against the same package when called.
import kitpkg

print(kitpkg.api())

from kitpkg import core

print(core is kitpkg.core)

from kitpkg.util import get

print(get() is core)
