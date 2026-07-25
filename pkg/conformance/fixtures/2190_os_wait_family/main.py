# The posix wait surface os.py re-exports (from posix import *) and subprocess
# drives to reap children: the W* macros decode a raw wait status,
# waitstatus_to_exitcode turns it into a shell-style code, os.pipe hands back
# non-inheritable fds, and set_inheritable/get_inheritable toggle the flag. The
# raw wait-status layout is identical on darwin and linux, so synthetic statuses
# keep the golden portable; the stop-signal number differs by platform, so that
# case is checked by predicate not by value.
import os
import signal

# A normal exit: low 7 bits clear, exit code in bits 8+.
st_exit = 42 << 8
print(os.WIFEXITED(st_exit), os.WIFSIGNALED(st_exit), os.WIFSTOPPED(st_exit))
print(os.WEXITSTATUS(st_exit))
print(os.waitstatus_to_exitcode(st_exit))

# Exit code 0 is a bare zero status.
print(os.WIFEXITED(0), os.WEXITSTATUS(0), os.waitstatus_to_exitcode(0))

# Killed by SIGTERM with no core dump: the term signal sits in the low 7 bits.
st_sig = signal.SIGTERM
print(os.WIFSIGNALED(st_sig), os.WIFEXITED(st_sig))
print(os.WTERMSIG(st_sig), os.WCOREDUMP(st_sig))
print(os.waitstatus_to_exitcode(st_sig))

# The core-dump bit (0x80) rides alongside the signal.
st_core = signal.SIGTERM | 0x80
print(os.WCOREDUMP(st_core), os.WTERMSIG(st_core))

# Stopped: low byte is 0x7f, stop signal in bits 8+. The number is
# platform-specific, so assert the predicate and that it round-trips.
st_stop = (int(signal.SIGSTOP) << 8) | 0x7f
print(os.WIFSTOPPED(st_stop), os.WIFEXITED(st_stop))
print(os.WSTOPSIG(st_stop) == signal.SIGSTOP)
try:
    os.waitstatus_to_exitcode(st_stop)
except ValueError:
    print("ValueError")

# os.pipe fds are non-inheritable, and set_inheritable toggles the flag.
r, w = os.pipe()
print(os.get_inheritable(r), os.get_inheritable(w))
os.set_inheritable(w, True)
print(os.get_inheritable(w))
os.set_inheritable(w, False)
print(os.get_inheritable(w))
os.close(r)
os.close(w)

# WNOHANG is a nonzero flag, and waiting on a pid that is not our child is
# ECHILD -> ChildProcessError.
print(os.WNOHANG != 0)
try:
    os.waitpid(2147483646, os.WNOHANG)
except ChildProcessError:
    print("ChildProcessError")
