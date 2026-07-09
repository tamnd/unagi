# yield from delegates to a sub-generator. At M4 the static tier has no
# generator state machine, so both generators stay boxed (doc 08 item 40), and
# the boxed tier must yield the delegated sequence then the tail value in the
# same order as CPython.
def sub():
    yield 1
    yield 2

def outer():
    yield from sub()
    yield 3

for x in outer():
    print(x)
