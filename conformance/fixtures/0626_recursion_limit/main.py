# Unbounded recursion raises a catchable RecursionError instead of crashing
# the interpreter, mirroring CPython's recursion-limit guard.

# Bounded recursion still returns normally.
def depth(n):
    if n == 0:
        return 0
    return 1 + depth(n - 1)

print("depth 100:", depth(100))

# Direct self-recursion trips the limit.
def loop():
    return loop()

try:
    loop()
except RecursionError as e:
    print("direct:", e.args[0])

# Mutual recursion trips the same shared limit.
def ping(n):
    return pong(n)

def pong(n):
    return ping(n)

try:
    ping(0)
except RecursionError:
    print("mutual caught")

# The depth counter unwinds with the caught error, so later calls still run.
print("still works:", depth(50))

# A recursive method trips too.
class Chain:
    def walk(self):
        return self.walk()

try:
    Chain().walk()
except RecursionError:
    print("method caught")

# A recursive lambda trips through its captured name.
rec = lambda n: rec(n)
try:
    rec(0)
except RecursionError:
    print("lambda caught")

# A recursive nested def trips inside its enclosing frame.
def outer():
    def inner(n):
        return inner(n)
    try:
        inner(0)
    except RecursionError:
        print("nested caught")

outer()
