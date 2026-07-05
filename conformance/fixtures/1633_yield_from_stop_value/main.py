# PEP 380: the value of a yield-from expression is the StopIteration value the
# delegated-to iterator finishes with. A sub-generator contributes its return
# value, a user iterator contributes the value its __next__ raises on
# StopIteration, and an iterator that carries no value finishes as None.


class Counter:
    def __init__(self, stop, result):
        self.n = 0
        self.stop = stop
        self.result = result

    def __iter__(self):
        return self

    def __next__(self):
        self.n += 1
        if self.n > self.stop:
            raise StopIteration(self.result)
        return self.n


def deleg_user():
    result = yield from Counter(3, "user-done")
    print("user result", result)


for v in deleg_user():
    print("u", v)


def sub():
    yield "a"
    yield "b"
    return "gen-done"


def deleg_gen():
    result = yield from sub()
    print("gen result", result)


for v in deleg_gen():
    print("g", v)


def deleg_list():
    result = yield from [10, 20]
    print("list result", result)


for v in deleg_list():
    print("l", v)


class Empty:
    def __iter__(self):
        return self

    def __next__(self):
        raise StopIteration


def deleg_empty():
    result = yield from Empty()
    print("empty result", result)


for v in deleg_empty():
    print("e", v)


# A next() on a user iterator still re-raises StopIteration carrying the value.
c = Counter(0, "carried")
try:
    next(c)
except StopIteration as e:
    print("next value", e.value)
