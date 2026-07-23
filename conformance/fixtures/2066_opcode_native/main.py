# _opcode is the introspection accelerator CPython ships in C. opcode.py opens
# with `import _opcode` and filters opmap through its has_* predicates, so opcode,
# and through it dis and the inspect chain, cannot import without it. The values
# below come straight from the 3.14.6 opcode grammar and are host independent.

import _opcode

# The has_* predicates classify an opcode number. Opcode 82 is LOAD_CONST, the
# lone has_const member the boxed tier keeps stable; opcode 3 is a specialized
# internal opcode with no opmap name, still a valid has_local member.
print("has_arg 82", _opcode.has_arg(82))
print("has_const 82", _opcode.has_const(82))
print("has_name 61", _opcode.has_name(61))
print("has_jump 68", _opcode.has_jump(68))
print("has_free 62", _opcode.has_free(62))
print("has_local 3", _opcode.has_local(3))
print("has_exc 263", _opcode.has_exc(263))
print("is_valid 3", _opcode.is_valid(3))
print("is_valid 12345", _opcode.is_valid(12345))

# The static descriptor tables.
print("intrinsic1 head", _opcode.get_intrinsic1_descs()[:3])
print("intrinsic2", _opcode.get_intrinsic2_descs())
print("special", _opcode.get_special_method_names())
print("nb_ops head", _opcode.get_nb_ops()[:2])
print("nb_ops tail", _opcode.get_nb_ops()[-1])

# opcode.py imports and builds its tables off the predicates.
import opcode

print("opcode hasarg", len(opcode.hasarg))
print("opcode hasconst", opcode.hasconst)
print("opcode hasname", len(opcode.hasname))
print("opcode hasjump", len(opcode.hasjump))
print("opcode LOAD_CONST", opcode.opmap["LOAD_CONST"])
