# PEP 562: a module-level __getattr__ serves names built on demand and declines
# anything else with AttributeError, exactly as typing.py does for Match/Pattern.
_built = {}


def __getattr__(name):
    if name in ("ALPHA", "BETA"):
        _built[name] = name.lower()
        return _built[name]
    raise AttributeError(f"module {__name__!r} has no attribute {name!r}")
