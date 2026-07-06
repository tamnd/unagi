# Implicit exception context is attached at raise time from the innermost
# exception being handled, and never rewritten on the way out of a handler.
# A finally unwinding an exception and an __exit__ running on the exception
# path both count as handling it while they run.


# A raise chain built across nested handlers survives the handler exits.
try:
    try:
        raise ValueError("A")
    except ValueError:
        try:
            raise KeyError("B")
        except KeyError:
            raise IndexError("C")
except IndexError as e:
    print(
        type(e).__name__,
        "->",
        type(e.__context__).__name__,
        "->",
        type(e.__context__.__context__).__name__,
    )


# A raise inside a finally chains onto the exception the finally unwinds.
try:
    try:
        raise ValueError("body")
    finally:
        raise KeyError("fin")
except KeyError as e:
    print("finally raise context:", type(e.__context__).__name__)


# A bare raise inside a finally re-raises the unwinding exception.
try:
    try:
        raise ValueError("unwound")
    finally:
        raise
except ValueError as e:
    print("finally bare raise:", e)


# With nothing unwinding, a bare raise in a finally sees the enclosing
# handler's exception.
try:
    raise ValueError("outer")
except ValueError:
    try:
        try:
            pass
        finally:
            raise
    except ValueError as e:
        print("idle finally bare raise:", e)


# A runtime-raised exception chains too: inside a finally over an in-flight
# exception, and inside a handler, picking the innermost handled exception.
try:
    try:
        raise ValueError("body2")
    finally:
        1 / 0
except ZeroDivisionError as e:
    print("runtime in finally:", type(e.__context__).__name__)

try:
    raise ValueError("h1")
except ValueError:
    try:
        raise KeyError("h2")
    except KeyError:
        try:
            1 / 0
        except ZeroDivisionError as e:
            print("runtime in handler:", type(e.__context__).__name__)


# A raising __exit__ chains the with-body exception even when an outer
# handler is active, and a bare raise inside __exit__ re-raises the body's.
class Raiser:
    def __enter__(self):
        return self

    def __exit__(self, et, ev, tb):
        raise KeyError("exit")


try:
    raise IndexError("outer-h")
except IndexError:
    try:
        with Raiser():
            raise ValueError("with-body")
    except KeyError as e:
        print("__exit__ raise context:", type(e.__context__).__name__)


class Bare:
    def __enter__(self):
        return self

    def __exit__(self, et, ev, tb):
        raise


try:
    with Bare():
        raise ValueError("bare-body")
except ValueError as e:
    print("__exit__ bare raise:", e)


# raise X from Y records the cause, keeps the context, and suppresses it.
try:
    try:
        raise ValueError("ctx")
    except ValueError:
        raise KeyError("k") from IndexError("cause")
except KeyError as e:
    print(
        "from:",
        type(e.__cause__).__name__,
        type(e.__context__).__name__,
        e.__suppress_context__,
    )


# Re-raising the same object under a different handler rewrites its context.
shared = KeyError("shared")
try:
    try:
        raise ValueError("first")
    except ValueError:
        raise shared
except KeyError:
    pass
print("first raise context:", type(shared.__context__).__name__)
try:
    try:
        raise IndexError("second")
    except IndexError:
        raise shared
except KeyError:
    pass
print("second raise context:", type(shared.__context__).__name__)


# next() on an exhausted iterator inside a handler chains the StopIteration.
try:
    raise ValueError("live")
except ValueError:
    try:
        next(iter([]))
    except StopIteration as e:
        print("StopIteration context:", type(e.__context__).__name__)
