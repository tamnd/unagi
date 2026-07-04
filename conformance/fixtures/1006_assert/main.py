try:
    assert 1 + 1 == 2
    print("assert passed")
    assert 2 > 3, "math broke"
except AssertionError as e:
    print("assert caught:", e)
try:
    assert False
except AssertionError as e:
    print("bare assert repr:", repr(e))
try:
    try:
        1 / 0
    except ZeroDivisionError:
        raise ValueError("clean slate") from None
except ValueError as e:
    print("from none:", e)
print("done")
