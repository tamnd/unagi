try:
    1 / 0
except ZeroDivisionError:
    raise RuntimeError("while handling")
