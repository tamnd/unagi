# _ast native module: import ast and exercise the node-type surface.
# No ast.parse here; the AOT world has no runtime parser, and the import
# chain only needs the node classes and the compile flags.
import ast

# The four names ast.py itself needs from _ast, all subclassable.
print(ast.AST.__name__, ast.mod.__name__, ast.expr_context.__name__, ast.Tuple.__name__)

# Construction binds positional args to _fields in order.
n = ast.Name('x', ast.Load())
print(type(n).__name__, n.id, type(n.ctx).__name__)

# Keyword construction and the field/attribute metadata.
c = ast.Constant(value=5, kind=None)
print(c.value, c.kind)
print(ast.Name._fields)
print(ast.Constant._fields)
print(ast.Module._fields)
print(ast.expr._attributes)
print(ast.operator._fields, ast.cmpop._fields)

# The abstract-base hierarchy and isinstance across it.
print(isinstance(n, ast.expr), isinstance(n, ast.AST))
print(isinstance(ast.Add(), ast.operator), isinstance(ast.Add(), ast.AST))
print(isinstance(ast.Lt(), ast.cmpop), isinstance(ast.USub(), ast.unaryop))
print(issubclass(ast.Name, ast.expr), issubclass(ast.expr, ast.AST))

# A nested node, and the field count round-trips.
t = ast.Tuple([ast.Name('a', ast.Load()), ast.Name('b', ast.Load())], ast.Load())
print(type(t).__name__, len(t.elts), t.elts[0].id, t.elts[1].id)

# All the operator nodes annotationlib constructs at import must be constructible.
ops = [ast.Add(), ast.Sub(), ast.Mult(), ast.MatMult(), ast.Div(), ast.Mod(),
       ast.LShift(), ast.RShift(), ast.BitOr(), ast.BitXor(), ast.BitAnd(),
       ast.FloorDiv(), ast.Pow(), ast.Invert(), ast.UAdd(), ast.USub()]
print([type(o).__name__ for o in ops])
cmps = [ast.Lt(), ast.LtE(), ast.Eq(), ast.NotEq(), ast.Gt(), ast.GtE()]
print([type(o).__name__ for o in cmps])

# The compile-mode flags.
print(ast.PyCF_ONLY_AST, ast.PyCF_TYPE_COMMENTS, ast.PyCF_ALLOW_TOP_LEVEL_AWAIT, ast.PyCF_OPTIMIZED_AST)

# Excess positional args raise, matching CPython.
try:
    ast.Name('a', ast.Load(), 'extra')
except TypeError as e:
    print('TypeError')
