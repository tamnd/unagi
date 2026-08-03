import builtins

# Writing into the builtins namespace at runtime makes a name reachable without
# an import, the way gettext.install exposes _ by doing
# builtins.__dict__['_'] = self.gettext. A read of that name from inside a
# function resolves to the injected value, the same fallback CPython runs when a
# name is in neither the local nor the module namespace.
builtins.__dict__["_"] = str.upper

# Injecting through setattr on the module reaches the same namespace.
setattr(builtins, "greet", lambda who: "hi " + who)


def translate():
    return _("hello")


def use_setattr():
    return greet("there")


def is_injected():
    return _("abc") == "ABC"


def read_undefined():
    return never_defined_name


def read_after_del():
    return _("gone")


print("inside func:", translate())
print("via setattr:", use_setattr())
print("injected works:", is_injected())

# A name in neither the module globals nor the builtins namespace still raises
# NameError, so the fallback does not turn every unknown name into a hit.
try:
    read_undefined()
except NameError as e:
    print("undefined raises:", type(e).__name__)

# Removing the injected name restores the undefined behaviour.
del builtins.__dict__["_"]
try:
    read_after_del()
except NameError as e:
    print("after del raises:", type(e).__name__)
