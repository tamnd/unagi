# A literal __all__ drives the star import: it binds each listed name in order,
# including an underscore name, and a name in __all__ the module never defined
# raises AttributeError after the earlier binds already took effect. A public
# name left out of __all__ is not bound at all.
try:
    from allmod import *
except AttributeError as e:
    print("error:", type(e).__name__, e)

print("a:", a)
print("_priv:", _priv)
try:
    print(b)
except NameError as e:
    print("b:", type(e).__name__, e)
