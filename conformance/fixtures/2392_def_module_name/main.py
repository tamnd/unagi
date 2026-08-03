# A module that reassigns __name__ (the pattern _pydecimal uses for pickling,
# `__name__ = 'decimal'`) must report the reassigned name as __module__ for the
# classes, functions, lambdas, and methods defined after it, matching CPython.
__name__ = "fakename"


class Klass:
    def method(self):
        pass


def func():
    pass


lam = lambda: 1


def outer():
    def nested():
        pass

    return nested


print("class      ->", Klass.__module__)
print("method     ->", Klass.method.__module__)
print("func       ->", func.__module__)
print("lambda     ->", lam.__module__)
print("nested     ->", outer().__module__)

import othermod

print("imp class  ->", othermod.C.__module__)
print("imp method ->", othermod.C.m.__module__)
print("imp func   ->", othermod.helper.__module__)
print("imp lambda ->", othermod.lam.__module__)
