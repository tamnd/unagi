"""The entry script's own docstring."""
# __doc__ is the module docstring, the value of a leading bare string literal
# and None otherwise. It stays an ordinary rebindable variable, so a later
# __doc__ = ... wins and an importer reads it back off the module object.
import withdoc
import nodoc
import reassign
import assignonly
import pkgdoc

print("main:", repr(__doc__))
print("withdoc:", repr(withdoc.__doc__))
print("nodoc:", repr(nodoc.__doc__))
print("reassign:", repr(reassign.__doc__))
print("assignonly:", repr(assignonly.__doc__))
print("pkgdoc:", repr(pkgdoc.__doc__))

# The imported module sees its own docstring under __doc__ too.
import selfread

print("selfread:", selfread.reported)

# Rebinding __doc__ in the entry module takes effect immediately.
__doc__ = "changed at runtime"
print("main after rebind:", repr(__doc__))
