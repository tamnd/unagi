# A function's __module__ is the module whose code ran its def: __main__ in the
# main module, the module's own __name__ in an imported module. Classes have
# always carried this; functions now do too. test.support.check__all__ compares
# __module__ against the module name to tell a module's own public functions from
# names it merely imported, so gettext.find must read back "gettext", not
# "__main__".
import gettext


def local_func():
    return 1


class Local:
    def method(self):
        return 2


square = lambda n: n * n

print("local func:", local_func.__module__)
print("local method:", Local.method.__module__)
print("local lambda:", square.__module__)
print("local class:", Local.__module__)

# A function, class and method defined in an imported module carry that module's
# name rather than the main module's.
print("gettext.find:", gettext.find.__module__)
print("gettext.translation:", gettext.translation.__module__)
print("gettext.NullTranslations:", gettext.NullTranslations.__module__)
print("NullTranslations.gettext:", gettext.NullTranslations.gettext.__module__)

# __module__ is a writable slot on a function, matching CPython.
local_func.__module__ = "reassigned"
print("after reassign:", local_func.__module__)
