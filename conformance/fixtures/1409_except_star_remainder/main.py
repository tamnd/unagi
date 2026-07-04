# The branches no clause matched propagate as a group, keeping the original
# message and structure.
try:
    raise ExceptionGroup("g", [ValueError("v"), TypeError("t"), KeyError("k")])
except* ValueError:
    print("handled VE")
except* TypeError:
    print("handled TE")
