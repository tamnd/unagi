# Every matching except* clause runs once, in the order written, and a fully
# handled group leaves nothing to propagate.
try:
    raise ExceptionGroup("g", [ValueError("v"), TypeError("t")])
except* ValueError:
    print("VE")
except* TypeError:
    print("TE")
print("after full handle")

# Clauses run in textual order, not in the group's order.
try:
    raise ExceptionGroup("g2", [ValueError("v"), TypeError("t")])
except* TypeError:
    print("TE first")
except* ValueError:
    print("VE second")

# A naked exception is wrapped so except* can match it.
try:
    raise ValueError("solo")
except* ValueError:
    print("caught solo")

# An unmatched clause is simply skipped when its type is absent.
try:
    raise ExceptionGroup("g3", [KeyError("k")])
except* KeyError:
    print("KE")
except* ValueError:
    print("unreached")
print("done")
