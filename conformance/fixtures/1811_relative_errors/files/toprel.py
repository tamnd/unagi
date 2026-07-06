try:
    from . import x
except ImportError as e:
    print("toprel:", type(e).__name__, e)
