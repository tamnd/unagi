# A builtin type constructor and the builtin root classes live in the builtins
# module, so their __module__ reads back "builtins", the way typing.py tests
# `origin.__module__ == 'builtins'` when it builds its special forms.

for t in (int, str, tuple, list, dict, float, bool, bytes, complex,
          set, frozenset, object, type):
    print(t.__name__, t.__module__, t.__qualname__)

print(BaseException.__module__, ValueError.__module__)


# A user class and a vendored-module class keep their own __module__.
class C:
    pass


print(C.__module__)

import abc

print(abc.ABC.__module__)
