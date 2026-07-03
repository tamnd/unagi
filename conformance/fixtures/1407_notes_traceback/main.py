# Uncaught chained exceptions with notes on both links.

def parse():
    raise ValueError("bad literal")

try:
    parse()
except ValueError as e:
    e.add_note("while reading config")
    wrapped = RuntimeError("config failed")
    wrapped.add_note("first note")
    wrapped.add_note("second\nnote")
    raise wrapped from e
