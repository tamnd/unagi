# A module that names most of its surface at runtime, the _constants shape:
# _makecodes injects the names through globals() and the module defines no
# __all__, so a star import must discover them at runtime.

def _makecodes(*names):
    globals().update({name: i for i, name in enumerate(names)})
    return list(names)

NAMES = _makecodes('ALPHA', 'BETA', 'GAMMA')
STATIC = 'kept'
_HIDDEN = 'skipped'
