def ret_swallows_return():
    try:
        return "try"
    finally:
        return "finally"


def ret_swallows_exception():
    try:
        raise ValueError("boom")
    finally:
        return "swallowed"


def break_swallows_return():
    for i in range(3):
        try:
            return "try-return"
        finally:
            break
    return "after-loop"


def continue_in_finally():
    out = []
    for i in range(3):
        try:
            if i == 1:
                raise RuntimeError("skip")
            out.append(("body", i))
        finally:
            continue
    return out


def break_wins_over_return():
    for i in range(5):
        try:
            return "never"
        finally:
            if i == 0:
                break
    return "loop-broke"


def nested_finally_return():
    try:
        try:
            return "inner-try"
        finally:
            return "inner-finally"
    finally:
        return "outer-finally"


print(ret_swallows_return())
print(ret_swallows_exception())
print(break_swallows_return())
print(continue_in_finally())
print(break_wins_over_return())
print(nested_finally_return())
