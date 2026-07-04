# A handler that raises combines with the unhandled remainder into one group,
# the raised exception first with the matched subgroup as its context.
def t5():
    try:
        raise ExceptionGroup("g", [ValueError("v"), KeyError("k")])
    except* ValueError:
        raise RuntimeError("boom")
t5()
