# os.get_terminal_size queries the terminal geometry through the TIOCGWINSZ
# ioctl and returns an os.terminal_size structseq; on a non-terminal fd it
# raises OSError, exactly what shutil.get_terminal_size catches to fall back on
# the COLUMNS/LINES environment. argparse builds its help formatter width this
# way, so unittest.main reaches it.
import os
import shutil

# terminal_size is a (columns, lines) structseq that constructs from a pair.
ts = os.terminal_size((80, 24))
print("repr", repr(ts))
print("fields", ts.columns, ts.lines)
print("index", ts[0], ts[1])
print("name", type(ts).__name__)
print("counts", ts.n_fields, ts.n_sequence_fields, ts.n_unnamed_fields)

# get_terminal_size on a non-terminal fd (a pipe) raises OSError.
r, w = os.pipe()
try:
    os.get_terminal_size(w)
    print("query", "no error")
except OSError:
    print("query", "OSError")
os.close(r)
os.close(w)

# shutil.get_terminal_size uses COLUMNS/LINES when set, without touching a tty.
os.environ["COLUMNS"] = "120"
os.environ["LINES"] = "40"
print("shutil", tuple(shutil.get_terminal_size()))
print("shutil-fallback", tuple(shutil.get_terminal_size((100, 30))))
