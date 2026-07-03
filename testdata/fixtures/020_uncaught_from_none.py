try:
    1 / 0
except ZeroDivisionError:
    raise RuntimeError("no context shown") from None
