# A package __init__ that imports its own submodule and re-exports a name
# from it, the common facade pattern; the submodule executes inside the
# package body and every later import is a cache hit.
import toolkit

print(toolkit.banner)
print(toolkit.shout("hi"))
print(toolkit.core.shout("there"))

import toolkit.core

print("once")

from toolkit import core

print(core is toolkit.core)
