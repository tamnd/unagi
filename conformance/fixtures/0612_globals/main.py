# A def reads a module-scope name that is only assigned later; the read
# resolves at call time, so late definition is fine.
def reader():
    return counter

counter = 10
print(reader())

# global lets a def rebind the module variable.
def bump():
    global counter
    counter = counter + 1

bump()
bump()
print(counter, reader())

# The value a def sees always tracks the current module binding, not any
# snapshot from when the def ran.
base = 1
def scaled():
    return base * 100

print(scaled())
base = 3
print(scaled())

# A global first bound inside a def becomes visible at module scope.
def install():
    global fresh
    fresh = "installed"

install()
print(fresh)

# global on a name never yet bound raises NameError only when read.
def read_missing():
    return absent

try:
    read_missing()
except NameError as e:
    print("missing:", e)

# del through global removes the module binding; later reads raise.
def drop():
    global counter
    del counter

drop()
try:
    reader()
except NameError as e:
    print("dropped:", e)

# Augmented assignment on a global reads then writes the module variable.
total = 0
def add(n):
    global total
    total += n

add(5)
add(7)
print(total)

# A comprehension inside a def still isolates its iteration variable even
# when a same-named global exists; the global keeps its value.
i = "module"
def loop():
    return [k for k in range(3)]

print(loop(), i)

# global with several names on one statement.
def setup():
    global a, b
    a = 1
    b = 2

setup()
print(a, b)

# A module variable shadows a like-named builtin when read from a def.
def which():
    return len

len = "shadowed"
print(which())

# Two defs share one global through reads and writes.
shared = []
def push(x):
    global shared
    shared = shared + [x]

def peek():
    return shared

push(1)
push(2)
print(peek())

# A module-scope function calling another module function that mutates a
# global observes the change.
state = "start"
def mutate():
    global state
    state = "changed"

def observe():
    return state

print(observe())
mutate()
print(observe())
