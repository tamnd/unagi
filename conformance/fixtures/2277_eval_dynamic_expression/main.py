# eval() evaluates a single Python expression parsed from a string at runtime.
# unagi compiles ahead of time with no interpreter, so a call to eval on a
# non-constant string runs the enclosing program boxed and hands the string to a
# runtime evaluator. Exercise the expression surface the vendored pure-Python
# stdlib relies on, most importantly the eval'd lambda that collections.namedtuple
# builds for each type's __new__.

# The source strings are built at runtime so this is genuine dynamic eval, not a
# constant the compiler could fold.
parts = ["1", "+", "2", "*", "3"]
print(eval(" ".join(parts)))

# Operators, comparisons, containers, and conditional expressions.
print(eval("2 ** 10 - 1"))
print(eval("7 // 2, 7 / 2, 7 % 2"))
print(eval("1 < 2 <= 2 < 3"))
print(eval("'z' if 3 > 4 else 'a'"))
print(eval("sorted([3, 1, 2])"))
print(eval("sum([1, 2, 3]) + max(4, 5)"))
print(eval("'ab' * 3 + 'c'"))
print(eval("2 in (1, 2, 3), 4 not in (1, 2, 3)"))

# A namespace: names resolve against the globals mapping, then the builtins.
ns = {"base": 10, "scale": 3}
print(eval("base * scale + len('abcd')", ns))

# The namedtuple pattern: eval a lambda whose parameters are chosen at runtime,
# capturing a helper from the namespace, then call it. This is exactly what
# collections.namedtuple does to synthesize each type's __new__.
field_names = ["x", "y", "z"]
arg_list = ", ".join(field_names)
namespace = {"_tuple_new": tuple}
code = "lambda " + arg_list + ": _tuple_new((" + arg_list + "))"
make = eval(code, namespace)
print(make(1, 2, 3))
print(make(z=30, x=10, y=20))

# A lambda default and a closed-over name from the eval namespace.
adder = eval("lambda a, b=bump: a + b", {"bump": 100})
print(adder(1))
print(adder(1, 2))

print("done")
