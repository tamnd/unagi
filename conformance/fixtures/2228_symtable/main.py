import symtable
import _symtable

# symtable.py reads only the flag constants at import; _symtable.symtable is
# called lazily inside its wrapper, so the constants alone let the module load.
print("imported")
print(symtable.__name__)

# The flag constants are the fixed values the C symtable ships, so a caller can
# unpack and test symbol flags exactly as CPython does.
print(_symtable.USE, _symtable.DEF_LOCAL, _symtable.DEF_PARAM, _symtable.DEF_GLOBAL)
print(_symtable.DEF_BOUND, _symtable.DEF_IMPORT, _symtable.DEF_ANNOT)
print(_symtable.SCOPE_OFF, _symtable.SCOPE_MASK)
print(_symtable.LOCAL, _symtable.GLOBAL_EXPLICIT, _symtable.GLOBAL_IMPLICIT, _symtable.FREE, _symtable.CELL)
print(_symtable.TYPE_MODULE, _symtable.TYPE_FUNCTION, _symtable.TYPE_CLASS)

# The scope of a symbol is packed into its flags at SCOPE_OFF, SCOPE_MASK wide;
# the constants let a caller decode it. A LOCAL symbol's flag word is exercised
# here to show the packing arithmetic is self-consistent.
flags = _symtable.DEF_LOCAL | (_symtable.LOCAL << _symtable.SCOPE_OFF)
print((flags >> _symtable.SCOPE_OFF) & _symtable.SCOPE_MASK == _symtable.LOCAL)

# The SymbolTable class hierarchy is defined and importable.
print(symtable.SymbolTable.__name__)
print(symtable.Symbol.__name__)

# symtable.symtable() itself compiles source, which needs a runtime parser the
# AOT build does not carry (the same wall as compile/exec and ast.parse), so it
# is exercised in the unit test rather than here where CPython would parse for
# real and diverge.
