# Reading a field name off a namedtuple class hands back its _tuplegetter
# descriptor, not a value. dis.py documents its _Instruction fields with
# `NT.field.__doc__ = "..."`, so the descriptor's __doc__ must be writable and
# its __get__ must read the field out of an instance.

import collections

NT = collections.namedtuple("NT", ["opname", "opcode"])

d = NT.opname
print("type", type(d).__name__)
print("repr", repr(d))
print("doc default", d.__doc__)

# __doc__ is writable and the write sticks on the shared descriptor.
NT.opname.__doc__ = "Human readable name for operation"
print("doc set", NT.opname.__doc__)
print("other field untouched", NT.opcode.__doc__)

# The descriptor reads the field out of an instance through __get__ (bound off
# the attribute, the descriptor protocol's own entry point).
inst = NT(11, 22)
get = NT.opname.__get__
print("get from instance", get(inst, NT))
print("field via attribute", inst.opname)

# dis.py builds _Instruction exactly this way and then sets a __doc__ on every
# field, so its import now goes through.
import dis

print("dis", dis.__name__)
print("has Instruction", hasattr(dis, "Instruction"))
