import builtins
import helpermod

# gettext.install exposes _ by writing builtins.__dict__['_'], and code in an
# imported module (test.test_gettext is one) then reads a bare _. The read
# happens in helpermod, a separate imported module, so it goes through the
# imported-module name path, not the main-module one; both take the same live
# builtins fallback. Model install with a stand-in so the read resolves.
builtins.__dict__["_"] = str.upper

print("imported-module func:", helpermod.translate())

# A name in neither the module globals nor builtins still raises NameError.
try:
    helpermod.read_undefined()
except NameError as e:
    print("undefined in module raises:", type(e).__name__)

# Removing the injected name restores the undefined behaviour inside the module.
del builtins.__dict__["_"]
try:
    helpermod.translate()
except NameError as e:
    print("after del in module raises:", type(e).__name__)
