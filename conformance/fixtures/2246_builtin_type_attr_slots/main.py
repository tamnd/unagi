# A builtin type inherits object's attribute-protocol slot wrappers
# (__getattribute__/__setattr__/__delattr__) at the object tail of its (T, object)
# MRO, exactly the way it inherits object's __repr__/__eq__ dunders. They read back
# as object's slot wrappers off the type and are callable as unbound slots, so
# T.__getattribute__(inst, name) runs object's generic read. unittest.mock's _Call
# subclasses tuple and reads self._mock_name through an explicit
# tuple.__getattribute__(self, name); without the inherited slot that call raised
# AttributeError and drove __getattr__ into unbounded recursion.

fn = lambda: 0
CodeType = type(fn.__code__)  # a constructor-less builtin type object

for T in (tuple, list, str, int, dict, CodeType):
    name = T.__name__
    for slot in ("__getattribute__", "__setattr__", "__delattr__"):
        print(name, slot, repr(getattr(T, slot)))

# The inherited slots are object's generic cores: calling them unbound through a
# builtin type performs the ordinary read/write/delete on an instance.
class Box:
    pass

b = Box()
tuple.__setattr__(b, "x", 10)          # any builtin type routes to object's slot
print("read", int.__getattribute__(b, "x"))
str.__delattr__(b, "x")
print("deleted", hasattr(b, "x"))

# The exact shape unittest.mock._Call leans on: a tuple subclass that overrides
# __getattribute__ to delegate to tuple.__getattribute__ and stores instance state
# set inside __new__. The instance attribute must be found by the inherited slot,
# not fall through to __getattr__.
class Call(tuple):
    def __new__(cls, seq, mock_name):
        self = tuple.__new__(cls, seq)
        self._mock_name = mock_name
        return self

    def __getattribute__(self, attr):
        if attr in tuple.__dict__:
            raise AttributeError
        return tuple.__getattribute__(self, attr)

    def __getattr__(self, attr):
        return ("via __getattr__", attr)


c = Call([1, 2], "root")
print("payload", tuple(c))
print("stored attr", c._mock_name)             # found by the inherited slot
print("in __dict__", "_mock_name" in c.__dict__)
print("missing attr", c.absent)                # falls through to __getattr__

# End-to-end: unittest.mock now imports and its core objects work, which is what
# the inherited slot unblocks.
from unittest.mock import Mock, MagicMock, patch, call

m = Mock(return_value=7)
print("mock call", m(1, 2, k=3))
m.assert_called_once_with(1, 2, k=3)
print("call_args", m.call_args)

mm = MagicMock()
mm.__len__.return_value = 3
print("magic len", len(mm))

import os
with patch("os.getpid", return_value=4242):
    print("patched", os.getpid())
print("call repr", call(1, x=2))
