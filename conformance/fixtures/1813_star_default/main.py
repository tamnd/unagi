# The default star rule binds every module-scope public name: plain values, a
# def, a class, and an imported module name all transfer, while underscore
# names stay behind. An existing binding the star names is overwritten, but a
# name the provider never actually bound (the dead if branch) is skipped, so
# the importer keeps its own value there.
visible = "original-visible"
ghost = "original-ghost"

from provider import *

print("helper:", helper.tag)
print("pub:", pub)
print("greet:", greet())
print("Widget:", Widget.kind)
print("visible:", visible)
print("ghost:", ghost)
try:
    print(_hidden)
except NameError as e:
    print("hidden:", type(e).__name__, e)
