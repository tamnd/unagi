try:
    from ... import nothing
except ImportError as e:
    print("beyond:", type(e).__name__, e)

try:
    from .missing import thing
except ImportError as e:
    print("relmiss:", type(e).__name__, e)

try:
    from . import gone
except ImportError as e:
    print("relattr:", type(e).__name__, e)
