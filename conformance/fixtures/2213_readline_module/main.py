# readline drives interactive line editing, history and completion. A compiled
# program is not the REPL readline edits, so the editing surface is a faithful
# no-op, but the history list, the completer and the completer delimiters are
# real state a program can depend on. Everything printed here is host-invariant
# (backend, which is "readline" on GNU and "editline" on libedit, is not
# printed).

import readline
import os

# History list: 1-based item access, replace, remove, clear.
readline.clear_history()
for w in ("alpha", "beta", "gamma", "delta"):
    readline.add_history(w)

print(readline.get_current_history_length())
print(readline.get_history_item(1), readline.get_history_item(4))
print(readline.get_history_item(0), readline.get_history_item(99))  # out of range -> None

readline.replace_history_item(1, "BETA")
print(readline.get_history_item(2))

readline.remove_history_item(0)
print(readline.get_current_history_length(), readline.get_history_item(1))

# History file round-trip: write the current history, clear, read it back.
path = "readline_hist.tmp"
readline.write_history_file(path)
readline.clear_history()
print(readline.get_current_history_length())
readline.read_history_file(path)
print(readline.get_current_history_length())
print([readline.get_history_item(i) for i in range(1, readline.get_current_history_length() + 1)])
os.remove(path)

# A missing history file raises FileNotFoundError.
try:
    readline.read_history_file("does_not_exist.tmp")
except FileNotFoundError:
    print("FileNotFoundError")

# set_history_length caps what write_history_file persists. Read the file back
# to observe the cap without depending on the on-disk format (libedit prepends a
# magic header that GNU readline does not).
readline.clear_history()
for i in range(5):
    readline.add_history("line%d" % i)
readline.set_history_length(2)
print(readline.get_history_length())
readline.write_history_file(path)
readline.clear_history()
readline.read_history_file(path)
print([readline.get_history_item(i) for i in range(1, readline.get_current_history_length() + 1)])
os.remove(path)

# Completer and delimiters round-trip.
def my_completer(text, state):
    return None

readline.set_completer(my_completer)
print(readline.get_completer() is my_completer)
default_delims = readline.get_completer_delims()
print(isinstance(default_delims, str) and len(default_delims) > 0)
readline.set_completer_delims(" \t\n")
print(readline.get_completer_delims())

# rlcompleter builds on the completer contract; its matches are deterministic.
import rlcompleter
c = rlcompleter.Completer({"spam": 1, "spanner": 2, "egg": 3})
print(c.complete("spa", 0), c.complete("spa", 1), c.complete("spa", 2))

# The degraded interactive surface: honest empties and no-ops.
print(repr(readline.get_line_buffer()))
print(readline.get_begidx(), readline.get_endidx())
print(readline.insert_text("ignored"))
print(readline.parse_and_bind("tab: complete"))
print(readline.redisplay())
readline.set_startup_hook()
readline.set_pre_input_hook(None)
readline.set_auto_history(True)
print("done")
