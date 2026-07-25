# The signal module runs on the native _signal accelerator: it star-imports the
# SIG* numbers, turns them into the Signals IntEnum and SIG_DFL/SIG_IGN into the
# Handlers enum, and wraps signal()/getsignal() so callers see enum members.
# Handlers install and read back, the two sentinels round-trip through the OS,
# an armed handler runs when the signal is raised, and the uncatchable signals
# and bad handler values raise the way CPython's do. Text that differs by OS
# (strsignal's exact phrasing) is checked by prefix so the output is the same on
# darwin and linux.
import signal
import time

# Constants are IntEnum members with the standard POSIX numbers.
print(int(signal.SIGINT), int(signal.SIGTERM), int(signal.SIGUSR1))
print(int(signal.SIG_DFL), int(signal.SIG_IGN))
print(type(signal.SIGINT).__name__, type(signal.SIG_IGN).__name__)
print(repr(signal.Signals(signal.SIGTERM)))
print(repr(signal.Handlers(1)))

# CPython seeds SIGINT with default_int_handler, a built-in function.
print(signal.getsignal(signal.SIGINT))

# A callable handler installs and reads back by identity, and returns the
# handler it replaced.
def handler(signum, frame):
    seen.append(signum)

seen = []
prev = signal.signal(signal.SIGUSR1, handler)
print(prev == signal.SIG_DFL)
print(signal.getsignal(signal.SIGUSR1) is handler)

# Raising the signal runs the handler; delivery is on a dispatch goroutine, so
# wait for it deterministically rather than assuming it already ran.
signal.raise_signal(signal.SIGUSR1)
for _ in range(200):
    if seen:
        break
    time.sleep(0.01)
print(seen == [int(signal.SIGUSR1)])

# SIG_IGN and SIG_DFL round-trip, handing the signal back to the OS.
old = signal.signal(signal.SIGUSR1, signal.SIG_IGN)
print(old is handler)
print(signal.getsignal(signal.SIGUSR1) == signal.SIG_IGN)
print(signal.signal(signal.SIGUSR1, signal.SIG_DFL) == signal.SIG_IGN)
print(signal.getsignal(signal.SIGUSR1) == signal.SIG_DFL)

# strsignal describes a signal; the phrasing's stable prefix is portable.
print(signal.strsignal(signal.SIGINT).startswith("Interrupt"))
print(signal.strsignal(signal.SIGTERM).startswith("Terminated"))

# valid_signals reports the platform's signals as a set of members.
vs = signal.valid_signals()
print(isinstance(vs, (set, frozenset)))
print(signal.SIGINT in vs and signal.SIGTERM in vs)

# The uncatchable signals raise OSError, and a non-callable non-sentinel handler
# raises TypeError, both matching CPython's wording.
try:
    signal.signal(signal.SIGKILL, signal.SIG_IGN)
except OSError as e:
    print("OSError", e)
try:
    signal.signal(signal.SIGINT, 5)
except TypeError as e:
    print("TypeError", e)
