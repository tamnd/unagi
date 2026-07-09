# A generator that receives values with `x = yield` is a bidirectional
# coroutine, which stays boxed at M4 (doc 08 item 41). The boxed tier must
# prime the generator with next(), then resume it with each sent value in the
# same order CPython does.
def echo():
    while True:
        got = yield
        print("got", got)

e = echo()
next(e)
e.send(10)
e.send(20)
