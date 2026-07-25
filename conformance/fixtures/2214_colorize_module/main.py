# _colorize backs the coloured output of traceback, argparse, unittest and the
# REPL. CPython builds its theme sections with @dataclass, an exec wall the AOT
# compiler cannot cross, so this is a hand-written native module. Everything
# printed here is host-invariant: colours are forced on or off explicitly rather
# than sniffed from a tty, so the output does not depend on where it runs.

import _colorize as c

# A theme is a set of sections; each section is a small mapping of role -> ANSI
# code. force_no_color yields a theme whose codes are all empty strings.
t = c.get_theme(force_no_color=True)
print(type(t).__name__, type(t.syntax).__name__)
print(t.syntax.comment == "", t.traceback.error_highlight == "")

# force_color yields real (non-empty) codes.
tc = c.get_theme(force_color=True)
print(tc.syntax.comment != "")

# A section is a mapping: len, iteration over field names, __getitem__ and a
# KeyError for an unknown role.
s = tc.syntax
print(len(s))
print(list(s) == [k for k in s])
print(s["comment"] == s.comment)
try:
    s["not_a_role"]
except KeyError:
    print("KeyError")

# copy_with returns a new section with selected roles overridden, leaving the
# original untouched. This exercises keyword dispatch on a native bound method.
s2 = s.copy_with(comment="OVERRIDE")
print(s2.comment, s.comment != "OVERRIDE")

# no_colors returns a same-typed section whose every role is the empty string.
nc = s.no_colors()
print(type(nc).__name__ == type(s).__name__)
print(all(nc[k] == "" for k in nc))

# get_colors returns the ANSIColors / NoColors palette, not a Theme.
print(type(c.get_colors(True)).__name__, type(c.get_colors(False)).__name__)
print(c.get_colors(False).RESET == "", c.get_colors(True).RESET != "")

# decolor strips ANSI escape sequences from a string.
print(repr(c.decolor("\x1b[35mviolet\x1b[0m and \x1b[1mbold\x1b[0m")))

# can_colorize answers a bool; with NO_COLOR forced it is False.
import os
os.environ["NO_COLOR"] = "1"
print(isinstance(c.can_colorize(), bool), c.can_colorize())
del os.environ["NO_COLOR"]

# set_theme installs a theme and type-checks its argument.
c.set_theme(c.get_theme(force_no_color=True))
print(c.get_theme() is not None)
try:
    c.set_theme("not a theme")
except ValueError as e:
    print("ValueError", str(e).startswith("Expected Theme object"))

# A real consumer: traceback formats an exception through _colorize. With colour
# off the message carries no escape codes.
import traceback
try:
    {}["missing"]
except KeyError as e:
    line = "".join(traceback.format_exception_only(type(e), e)).strip()
    print(line, "\x1b[" not in line)

print("done")
