import termios
import tty
import pty
import os

# termios is the terminal-attributes accelerator tty and pty drive raw and
# cbreak mode through. The round-trip (tcgetattr -> edit -> tcsetattr) needs a
# real terminal, so it is checked separately under a pty; here only the
# host-invariant surface and the non-terminal error path are exercised, both of
# which the corpus runner (no pty on stdin) reproduces the same as the oracle.

# The when-constants are fixed by POSIX at 0, 1, 2 on every host.
print("when:", termios.TCSANOW, termios.TCSADRAIN, termios.TCSAFLUSH)

# tty re-exports the four helpers it builds on termios.
print("tty.__all__:", sorted(tty.__all__))

# error is a distinct Exception subclass, the class tcgetattr/tcsetattr raise.
print("error subclass of Exception:", issubclass(termios.error, Exception))

# The flag bits and cc indices are host-specific numbers, so their exact values
# are not printed; only that each is present as a nonzero int (the cc indices
# VMIN/VTIME may legitimately be zero on a host, so they are checked as ints).
flags = ["ECHO", "ICANON", "ISIG", "IEXTEN", "OPOST", "PARENB", "CS8", "CSIZE"]
vals = [getattr(termios, n) for n in flags]
print("flags all nonzero ints:", all(isinstance(v, int) and v != 0 for v in vals))
print("VMIN/VTIME ints:", isinstance(termios.VMIN, int), isinstance(termios.VTIME, int))

# tcgetattr on a descriptor that is not a terminal raises termios.error. A pipe
# read-end is never a tty, so this is the failure both unagi and CPython report.
# The error message is host-specific, so only the exception type is printed.
r, w = os.pipe()
try:
    termios.tcgetattr(r)
    print("tcgetattr(pipe): no error (unexpected)")
except termios.error:
    print("tcgetattr(pipe): termios.error")
except Exception as e:
    print("tcgetattr(pipe): wrong exception", type(e).__name__)
os.close(r)
os.close(w)

# pty exposes the standard descriptor numbers it copies between.
print("pty STDIN_FILENO:", pty.STDIN_FILENO)
