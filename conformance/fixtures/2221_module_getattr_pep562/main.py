import lazy

# Attribute access routes a missing name through the module __getattr__.
print("attr", lazy.ALPHA)
# A repeat access returns the same lazily built value.
print("attr again", lazy.BETA)
# from-import routes through the same hook.
from lazy import ALPHA
print("from", ALPHA)
# A name the hook declines raises AttributeError on attribute access.
try:
    lazy.GAMMA
except AttributeError as e:
    print("attr decline", e)
# and ImportError on from-import.
try:
    from lazy import GAMMA
except ImportError as e:
    print("from decline", type(e).__name__)

# typing relies on PEP 562 to build the soft-deprecated Match and Pattern
# aliases on demand, reading re.Match.__module__ as it goes.
from typing import Match, Pattern
import typing

print("Match", Match)
print("Pattern", Pattern)
print("ContextManager", typing.ContextManager)
print("re.Match module", Match.__origin__.__module__)
try:
    typing.NoSuchName
except AttributeError as e:
    print("typing decline", e)
