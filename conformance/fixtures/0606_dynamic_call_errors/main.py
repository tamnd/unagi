# Binding and callability errors on dynamic calls, where the runtime binder
# owns the TypeError. Wordings probed on 3.14.

def f(a, /, count=1, *, flag=True):
    return (a, count, flag)

g = f
try:
    g()
except TypeError as e:
    print(e)
try:
    g(1, 2, 3)
except TypeError as e:
    print(e)
try:
    g(1, 2, 3, flag=False)
except TypeError as e:
    print(e)
try:
    g(1, a=2)
except TypeError as e:
    print(e)
try:
    g(1, cout=2)
except TypeError as e:
    print(e)
try:
    g(1, 2, count=3)
except TypeError as e:
    print(e)

def need(a, b, *, k, kk):
    return a

h = need
try:
    h(1)
except TypeError as e:
    print(e)
try:
    h(1, 2)
except TypeError as e:
    print(e)

lam = lambda x, *, k: (x, k)
try:
    lam(1, 2)
except TypeError as e:
    print(e)
try:
    lam(1)
except TypeError as e:
    print(e)

def outer():
    inner = lambda p: p
    return inner

try:
    outer()(1, 2)
except TypeError as e:
    print(e)

for bad in [3, "s", None, 1.5, [1], (1,), {1: 2}]:
    try:
        bad()
    except TypeError as e:
        print(e)

lt = lambda: 0
gt = lambda: 0
try:
    print(lt < gt)
except TypeError as e:
    print(e)
try:
    print(lt <= lt)
except TypeError as e:
    print(e)
