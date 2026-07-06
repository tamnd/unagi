# The fromlist split: `from box import inner` finds no attribute and falls
# back to importing the submodule, which also binds it on the package. An
# attribute assigned by __init__ wins over a same-named submodule until a
# direct import of that submodule overwrites it.
from box import inner

import box

print(inner is box.inner)
print(inner.knob)

from box import shadow, tag

print(shadow, "/", tag)

import box.shadow

print(box.shadow.knob, type(box.shadow).__name__)

from box import shadow

print(shadow is box.shadow)

from box.inner import knob as k

print(k)
