# A yield may sit inside try, except, else, finally, and with. The generator
# stashes the exception entries its own frame is handling whenever it
# suspends and restores them on resume, so the consumer's exception state and
# the generator's never bleed into each other.


# close() drives GeneratorExit through the finally.
def fin():
    try:
        yield 1
        yield 2
    finally:
        print("fin: finally")


it = fin()
print(next(it))
it.close()
print("fin: closed")


# throw() lands at the yield, the except catches it, the body yields again.
def catcher():
    try:
        yield "a"
    except ValueError as e:
        print("catcher caught", e)
        yield "b"


it = catcher()
print(next(it))
print(it.throw(ValueError("boom")))
try:
    next(it)
except StopIteration:
    print("catcher done")


# A generator suspended inside its own except block keeps that exception as
# its raise context even when the consumer handles other exceptions in
# between, and a consumer bare raise does not see the generator's exception.
def holder():
    try:
        raise ValueError("held")
    except ValueError:
        yield 1
        raise KeyError("after")


it = holder()
print(next(it))
try:
    raise
except RuntimeError as e:
    print("consumer bare raise:", e)
try:
    try:
        raise IndexError("consumer")
    except IndexError:
        next(it)
except KeyError as e:
    print("holder context:", type(e.__context__).__name__)


# A bare raise inside the generator after the suspension re-raises its own
# handled exception.
def bare():
    try:
        raise OSError("os")
    except OSError:
        yield 1
        raise


it = bare()
print(next(it))
try:
    next(it)
except OSError as e:
    print("bare re-raise:", e)


# Two generators suspended inside their own handlers interleave cleanly.
def tagged(tag):
    try:
        raise ValueError(tag)
    except ValueError:
        yield tag + "1"
        try:
            raise KeyError(tag)
        except KeyError:
            yield tag + "2"
        raise


a, b = tagged("a"), tagged("b")
print(next(a), next(b), next(a), next(b))
for it, tag in ((a, "a"), (b, "b")):
    try:
        next(it)
    except ValueError as e:
        print("tagged", tag, "->", e)


# yield inside with: close() runs __exit__ with GeneratorExit.
class CM:
    def __enter__(self):
        print("cm enter")
        return self

    def __exit__(self, et, ev, tb):
        print("cm exit", et.__name__ if et else None)
        return False


def managed():
    with CM():
        yield 1
        yield 2


it = managed()
print(next(it))
it.close()
print("managed closed")


# return inside try still runs the finally and carries the value.
def valued():
    try:
        yield 1
        return 42
    finally:
        print("valued finally")


it = valued()
print(next(it))
try:
    next(it)
except StopIteration as e:
    print("valued value:", e.value)


# Catching GeneratorExit and returning is a clean close; yielding after it
# is the RuntimeError.
def polite():
    try:
        yield 1
    except GeneratorExit:
        print("polite: caught exit")
        return


it = polite()
print(next(it))
it.close()
print("polite closed")


def rude():
    try:
        yield 1
    except GeneratorExit:
        yield 2


it = rude()
print(next(it))
try:
    it.close()
except RuntimeError as e:
    print("rude:", e)


# An outer generator holding a live handler entry delegates to an inner one
# that raises; the inner raise chains onto the outer's handled exception.
def inner():
    yield 1
    raise KeyError("ink")


def outer():
    try:
        raise ValueError("outv")
    except ValueError:
        yield from inner()


it = outer()
print(next(it))
try:
    next(it)
except KeyError as e:
    print("delegated context:", type(e.__context__).__name__)


# else and finally around a yield, driven to completion.
def full():
    try:
        yield 1
    except ValueError:
        print("full: wrong branch")
    else:
        print("full: else")
        yield 2
    finally:
        print("full: finally")


print(list(full()))
