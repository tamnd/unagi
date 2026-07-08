# The decode form of the str constructor, str(object, encoding[, errors]).
print(str(b'abc', 'latin1'))
print(str(b'caf\xc3\xa9', 'utf-8'))
print(str(bytearray(b'hi there'), 'ascii'))
print(str(memoryview(b'view'), 'utf-8'))
print(str(b'a\xffc', 'ascii', 'replace'))
print(str(b'a\xffc', 'ascii', 'ignore'))
print(repr(str(b'', 'utf-8')))

# The one-argument and zero-argument forms still stand.
print(str())
print(str(42))
print(str(b'raw'))


def show(thunk):
    try:
        thunk()
    except (TypeError, LookupError, ValueError) as exc:
        print(type(exc).__name__, exc)


show(lambda: str(1, 'utf-8'))
show(lambda: str('already', 'utf-8'))
show(lambda: str(b'x', 5))
show(lambda: str(b'x', 'utf-8', 5))
show(lambda: str(b'x', 'no-such-codec'))
show(lambda: str(b'a\xffc', 'ascii'))
