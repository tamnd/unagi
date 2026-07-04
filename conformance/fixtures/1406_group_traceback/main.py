# Uncaught exception group: the box rendering with nested groups, notes,
# and a sub-exception that carries its own context chain.

def collect():
    try:
        raise KeyError("inner")
    except KeyError:
        raise ValueError("chained")

inner = ExceptionGroup("in", [KeyError("k"), OSError(9)])
inner.add_note("group note")
try:
    collect()
except ValueError as e:
    top = ExceptionGroup("out", [ValueError(1), inner, e])
    top.add_note("top note 1")
    top.add_note("multi\nline note")
    raise top
