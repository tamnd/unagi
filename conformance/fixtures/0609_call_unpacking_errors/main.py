# Unpacking error catalog: the two star wordings, the ** mapping check,
# duplicate keywords across sources, non-str keys, check timing against
# argument evaluation, and binder errors reached through unpacking. All
# wordings probed on 3.14.

def f(a, b=2, *rest, k=1, **extra):
    print(a, b, rest, k, extra)

def mk(tag, v):
    print("eval", tag)
    return v

# Lone star converts at call time and names the callee.
try:
    f(*3)
except TypeError as e:
    print(e)

# A star among other positional parts uses the name-free wording and fires
# before anything to its right evaluates.
try:
    f(mk("p1", 1), *mk("bad", 3), mk("p2", 2))
except TypeError as e:
    print(e)

# ** checks mapping-ness in argument position.
try:
    f(**3)
except TypeError as e:
    print(e)
try:
    f(**[("a", 1)])
except TypeError as e:
    print(e)
try:
    f(**mk("m", 3), k=mk("never", 1))
except TypeError as e:
    print(e)

# Duplicates between keyword sources.
try:
    f(1, k=1, **{"k": 2})
except TypeError as e:
    print(e)
try:
    f(1, **{"k": 1}, **{"k": 2})
except TypeError as e:
    print(e)

# Non-str keys survive the merge and only die at call time; a failing
# lone-star conversion outranks them.
try:
    f(*[1], **{1: 2})
except TypeError as e:
    print(e)
try:
    f(*3, **{1: 2})
except TypeError as e:
    print(e)

# Binder errors keep their bare spelling through the unpacking path.
try:
    f()
except TypeError as e:
    print(e)
try:
    f(*[1, 2], b=9)
except TypeError as e:
    print(e)
try:
    f(*[1], q=9)
except TypeError as e:
    print(e)

# Lambdas and builtins spell their callee their own way.
try:
    (lambda x: x)(*3)
except TypeError as e:
    print(e)
try:
    print(*3)
except TypeError as e:
    print(e)
try:
    len(*[1, 2])
except TypeError as e:
    print(e)
try:
    len(*[[1], [2]])
except TypeError as e:
    print(e)
try:
    sorted(*[[1], [2]])
except TypeError as e:
    print(e)
try:
    divmod(*[1])
except TypeError as e:
    print(e)

# Methods and exception constructors.
lst = []
try:
    lst.append(*5)
except TypeError as e:
    print(e)
try:
    ValueError(*3)
except TypeError as e:
    print(e)

# Calling a non-callable through the unpacking path.
try:
    n = 3
    n(*[1])
except TypeError as e:
    print(e)
